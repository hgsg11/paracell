package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"sort"
	"strings"
)

// ContainerTemplateKey identifies one container declaration of one TargetCell.
// The same container name may be declared by several TargetCells.
type ContainerTemplateKey struct {
	Target    string
	Container string
}

// CellResourceLayout holds the names and paths Paracell uses for the resources
// of one CellGroup. Template names do not have to be unique once they are made
// safe for filesystems and Docker; the layout absorbs those collisions. Names
// that do not collide keep their plain form, so existing Cells keep their names.
type CellResourceLayout struct {
	cellDirectory  string
	targetSegments map[string]string
	sourceOwners   map[string]string
	sourcePaths    map[string]string
	containerNames map[ContainerTemplateKey]string
	containerRoles map[ContainerTemplateKey]string
}

// BuildCellResourceLayoutService decides the resource names of a CellGroup from
// its TargetCells. The result only depends on the target and container names,
// so creation and clean derive the same names from the persisted TargetCells.
func BuildCellResourceLayoutService(group CellGroup, targets []TargetCell) CellResourceLayout {
	sorted := append([]TargetCell(nil), targets...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	layout := CellResourceLayout{
		cellDirectory:  filepath.Join(".paracell", "cells", group.Name().Value),
		targetSegments: make(map[string]string, len(sorted)),
		sourceOwners:   make(map[string]string, len(sorted)),
		sourcePaths:    make(map[string]string, len(sorted)),
		containerNames: make(map[ContainerTemplateKey]string),
		containerRoles: make(map[ContainerTemplateKey]string),
	}

	targetCandidates := make([]nameCandidate, 0, len(sorted))
	for _, target := range sorted {
		targetCandidates = append(targetCandidates, nameCandidate{identity: target.Name, base: SafeResourceName(target.Name, "target")})
	}
	for identity, name := range resolveNameCollisions(targetCandidates) {
		layout.targetSegments[identity] = name
	}

	owners := make(map[string]string, len(sorted))
	for _, target := range sorted {
		if target.Source == nil {
			continue
		}
		layout.sourcePaths[target.Name] = target.Source.Path
		owner, exists := owners[target.Source.Path]
		if !exists {
			owner = target.Name
			owners[target.Source.Path] = owner
		}
		layout.sourceOwners[target.Name] = owner
	}

	var nameCandidates, roleCandidates []nameCandidate
	keys := make(map[string]ContainerTemplateKey)
	for _, target := range sorted {
		for _, container := range target.Containers {
			key := ContainerTemplateKey{Target: target.Name, Container: container.SourceContainer}
			identity := target.Name + "/" + container.SourceContainer
			keys[identity] = key
			role := SafeResourceName(container.SourceContainer, "service")
			nameCandidates = append(nameCandidates, nameCandidate{
				identity: identity,
				base:     group.ResourcePrefix() + "-" + layout.targetSegments[target.Name] + "-" + SafeResourceName(container.SourceContainer, "container"),
			})
			roleCandidates = append(roleCandidates, nameCandidate{
				identity:  identity,
				base:      role,
				qualified: role + "." + layout.targetSegments[target.Name],
			})
		}
	}
	for identity, name := range resolveNameCollisions(nameCandidates) {
		layout.containerNames[keys[identity]] = name
	}
	for identity, role := range resolveNameCollisions(roleCandidates) {
		layout.containerRoles[keys[identity]] = role
	}
	return layout
}

// CellDirectory is the directory that holds the TargetCell directories.
func (l CellResourceLayout) CellDirectory() string { return l.cellDirectory }

// TargetDirectory is the directory of one TargetCell.
func (l CellResourceLayout) TargetDirectory(target string) string {
	segment, ok := l.targetSegments[target]
	if !ok {
		segment = SafeResourceName(target, "target")
	}
	return filepath.Join(l.cellDirectory, segment)
}

// SourceOwner is the TargetCell whose directory holds the worktree used by
// target. TargetCells with the same source path share one worktree, because a
// CellGroup has a single branch.
func (l CellResourceLayout) SourceOwner(target string) string {
	if owner, ok := l.sourceOwners[target]; ok {
		return owner
	}
	return target
}

// SourceRoot is the "source" directory of target. For a TargetCell that does
// not own its worktree, it is the path of the link to the owner's directory.
func (l CellResourceLayout) SourceRoot(target string) string {
	return filepath.Join(l.TargetDirectory(target), "source")
}

// SourceWorktreePath is the worktree that target works on. TargetCells sharing
// a source path get the owner's worktree.
func (l CellResourceLayout) SourceWorktreePath(target string) string {
	path := l.SourceRoot(l.SourceOwner(target))
	if sourcePath := l.sourcePaths[target]; sourcePath != "" && sourcePath != "." {
		path = filepath.Join(path, sourcePath)
	}
	return path
}

// ContainerName is the Docker container name of one TargetCell container.
func (l CellResourceLayout) ContainerName(target string, container string) string {
	return l.containerNames[ContainerTemplateKey{Target: target, Container: container}]
}

// ContainerRole is the service role used in the gateway host of one TargetCell
// container. It stays the container name unless another container of the
// CellGroup would get the same host.
func (l CellResourceLayout) ContainerRole(target string, container string) string {
	return l.containerRoles[ContainerTemplateKey{Target: target, Container: container}]
}

type nameCandidate struct {
	identity  string
	base      string
	qualified string
}

// resolveNameCollisions keeps base names that are unique when compared case
// insensitively. Colliding names first use their qualified form when one is
// given, and names that still collide get a hash of their identity.
func resolveNameCollisions(candidates []nameCandidate) map[string]string {
	names := make(map[string]string, len(candidates))
	for _, candidate := range candidates {
		names[candidate.identity] = candidate.base
	}
	if hasQualified(candidates) {
		for _, identity := range collidingIdentities(candidates, names) {
			for _, candidate := range candidates {
				if candidate.identity == identity && candidate.qualified != "" {
					names[identity] = candidate.qualified
				}
			}
		}
	}
	for _, identity := range collidingIdentities(candidates, names) {
		names[identity] = names[identity] + "-" + shortHash(identity)
	}
	return names
}

func hasQualified(candidates []nameCandidate) bool {
	for _, candidate := range candidates {
		if candidate.qualified != "" {
			return true
		}
	}
	return false
}

func collidingIdentities(candidates []nameCandidate, names map[string]string) []string {
	counts := make(map[string]int, len(candidates))
	for _, candidate := range candidates {
		counts[strings.ToLower(names[candidate.identity])]++
	}
	var colliding []string
	for _, candidate := range candidates {
		if counts[strings.ToLower(names[candidate.identity])] > 1 {
			colliding = append(colliding, candidate.identity)
		}
	}
	return colliding
}

func shortHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:8]
}
