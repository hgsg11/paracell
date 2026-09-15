package domain

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

type Cell struct {
	Version            CellVersion
	ID                 string
	Issue              string
	Project            string
	Note               string
	Template           string
	Sources            Sources
	Containers         Containers
	Session            Session
	NotificationDriver NotificationDriverType
	Creation           CellCreation
	Status             CellStatus
	Done               bool
}

type CellVersion uint64

func NewCellVersion(value uint64) (CellVersion, error) {
	if value == 0 {
		return 0, fmt.Errorf("cell version must be greater than zero")
	}
	return CellVersion(value), nil
}

func NewCell(id string, issue string, project string, templateName string, sources Sources, containers Containers, session Session, notificationDriver NotificationDriverType) (Cell, error) {
	if id == "" {
		return Cell{}, fmt.Errorf("cell id is required")
	}
	if issue == "" {
		return Cell{}, fmt.Errorf("issue is required")
	}
	if templateName == "" {
		return Cell{}, fmt.Errorf("template name is required")
	}
	version, err := NewCellVersion(1)
	if err != nil {
		return Cell{}, err
	}
	return Cell{
		Version: version, ID: id, Issue: issue, Project: project,
		Template: templateName, Sources: sources, Containers: containers, Session: session, NotificationDriver: notificationDriver,
		Creation: NewCellCreation(), Status: Ready,
	}, nil
}

type CreationStatus string

const (
	CreationCreating CreationStatus = "creating"
	CreationFailed   CreationStatus = "failed"
	CreationRetrying CreationStatus = "retrying"
	CreationReady    CreationStatus = "ready"
)

func NewCreationStatus(value string) (CreationStatus, error) {
	status := CreationStatus(value)
	switch status {
	case CreationCreating, CreationFailed, CreationRetrying, CreationReady:
		return status, nil
	default:
		return status, fmt.Errorf("invalid creation status %q", status)
	}
}

type CreationStage string

const (
	CreationStageSource     CreationStage = "source"
	CreationStageContainers CreationStage = "containers"
	CreationStageSession    CreationStage = "session"
)

func NewCreationStage(value string) (CreationStage, error) {
	stage := CreationStage(value)
	switch stage {
	case CreationStageSource, CreationStageContainers, CreationStageSession:
		return stage, nil
	default:
		return stage, fmt.Errorf("invalid creation stage %q", stage)
	}
}

type CellCreation struct {
	Status           CreationStatus
	Command          string
	CompletedStages  []CreationStage
	FailedStage      CreationStage
	LastError        string
	AttemptID        string
	LeaseStartedAt   *time.Time
	LeaseHeartbeatAt *time.Time
}

func NewCellCreation() CellCreation {
	return CellCreation{Status: CreationReady}
}

type CellStatus string

const (
	Pending CellStatus = "pending"
	Ready   CellStatus = "ready"
)

func NewCellStatus(value string) (CellStatus, error) {
	status := CellStatus(value)
	switch status {
	case Pending, Ready:
		return status, nil
	default:
		return status, fmt.Errorf("unsupported status %q", status)
	}
}

type Sources struct {
	Driver SourceDriverType
	Items  []Source
}

func NewSources(driver SourceDriverType, items []Source) Sources {
	return Sources{Driver: driver, Items: append([]Source(nil), items...)}
}

type Source struct {
	Path   string
	Base   string
	Branch string
}

func NewSource(path string, base string, branch string) (Source, error) {
	template, err := NewSourceTemplate(path, base, "")
	if err != nil {
		return Source{}, err
	}
	if branch == "" {
		return Source{}, fmt.Errorf("source branch is required")
	}
	return Source{Path: template.Path, Base: base, Branch: branch}, nil
}

type Containers struct {
	Driver ContainerDriverType
	Items  []Container
}

func NewContainers(driver ContainerDriverType, items []Container) Containers {
	return Containers{Driver: driver, Items: append([]Container(nil), items...)}
}

type Container struct {
	Role            string
	Network         []string
	SourceContainer string
	Mode            Mode
}

func NewContainer(role string, network []string, sourceContainer string, mode Mode) (Container, error) {
	if role == "" {
		return Container{}, fmt.Errorf("container role is required")
	}
	if sourceContainer == "" {
		return Container{}, fmt.Errorf("source container is required")
	}
	validatedMode, err := NewMode(string(mode))
	if err != nil {
		return Container{}, err
	}
	return Container{Role: role, Network: append([]string(nil), network...), SourceContainer: sourceContainer, Mode: validatedMode}, nil
}

type Session struct {
	Driver  SessionDriverType
	Windows []SessionWindow
}

func NewSession(driver SessionDriverType, windows []SessionWindow) Session {
	return Session{Driver: driver, Windows: append([]SessionWindow(nil), windows...)}
}

type SessionWindow struct {
	Name    string
	Command string
}

func NewSessionWindow(name string, command string) (SessionWindow, error) {
	if name == "" {
		return SessionWindow{}, fmt.Errorf("session window name is required")
	}
	return SessionWindow{Name: name, Command: command}, nil
}

func BuildSources(driver SourceDriverType, templates []SourceTemplate, issue string) (Sources, error) {
	items := make([]Source, 0, len(templates))
	for _, template := range templates {
		source, err := NewSource(template.Path, template.Base, template.Prefix+issue)
		if err != nil {
			return Sources{}, err
		}
		items = append(items, source)
	}
	return NewSources(driver, items), nil
}

func BuildContainers(driver ContainerDriverType, templates []ContainerTemplate) (Containers, error) {
	items := make([]Container, 0, len(templates))
	for _, template := range templates {
		container, err := NewContainer(template.Name, nil, template.Name, template.Mode)
		if err != nil {
			return Containers{}, err
		}
		items = append(items, container)
	}
	return NewContainers(driver, items), nil
}

func BuildSession(driver SessionDriverType, template SessionTemplate) (Session, error) {
	windows := make([]SessionWindow, 0, len(template.Windows))
	for _, item := range template.Windows {
		window, err := NewSessionWindow(item.Name, item.Command)
		if err != nil {
			return Session{}, err
		}
		windows = append(windows, window)
	}
	return NewSession(driver, windows), nil
}

func BuildSourcesForRetry(stored Cell, driver SourceDriverType, templates []SourceTemplate, issue string) (Sources, error) {
	if CellCreationStageCompleted(stored, CreationStageSource) {
		return CloneCell(stored).Sources, nil
	}
	return BuildSources(driver, templates, issue)
}

func BuildContainersForRetry(stored Cell, driver ContainerDriverType, templates []ContainerTemplate) (Containers, error) {
	if CellCreationStageCompleted(stored, CreationStageContainers) {
		return CloneCell(stored).Containers, nil
	}
	return BuildContainers(driver, templates)
}

func BuildSessionForRetry(stored Cell, driver SessionDriverType, template SessionTemplate) (Session, error) {
	if CellCreationStageCompleted(stored, CreationStageSession) {
		return CloneCell(stored).Session, nil
	}
	return BuildSession(driver, template)
}

func CellName(cell Cell) string {
	return SafeResourceName(cell.Issue, cell.ID)
}

func CellResourcePrefix(cell Cell) string {
	return fmt.Sprintf("paracell-%s-%s", SafeResourceName(cell.Project, "project"), CellName(cell))
}

func SourceWorktreePath(cell Cell, source Source) string {
	path := filepath.Join(".paracell", "cells", CellName(cell), "source")
	if source.Path != "." {
		path = filepath.Join(path, source.Path)
	}
	return path
}

func ContainerNetworkName(cell Cell) string {
	return CellResourcePrefix(cell)
}

func ContainerResourceName(cell Cell, container Container) string {
	if container.Mode == Dependency {
		return container.SourceContainer
	}
	return CellResourcePrefix(cell) + "-" + SafeResourceName(container.Role, "container")
}

func SessionName(cell Cell) string {
	return SafeResourceName(cell.Project, "project") + "-" + CellName(cell)
}

func CellDisplayLabel(cell Cell) string {
	if cell.Note != "" {
		return cell.Note
	}
	return CellName(cell)
}

type CellSummary struct {
	Version        CellVersion
	ID             string
	Issue          string
	Name           string
	DisplayLabel   string
	Template       string
	CreationStatus CreationStatus
	Status         CellStatus
	Done           bool
	FailedStage    CreationStage
	LastError      string
}

func NewCellSummary(version CellVersion, id string, issue string, name string, displayLabel string, templateName string, creationStatus CreationStatus, status CellStatus, done bool, failedStage CreationStage, lastError string) CellSummary {
	return CellSummary{Version: version, ID: id, Issue: issue, Name: name, DisplayLabel: displayLabel, Template: templateName, CreationStatus: creationStatus, Status: status, Done: done, FailedStage: failedStage, LastError: lastError}
}

func SummarizeCell(cell Cell) CellSummary {
	return NewCellSummary(cell.Version, cell.ID, cell.Issue, CellName(cell), CellDisplayLabel(cell), cell.Template, CellCreationStatus(cell), cell.Status, cell.Done, cell.Creation.FailedStage, cell.Creation.LastError)
}

type CellDrivers struct {
	Source       SourceDriverType
	Container    ContainerDriverType
	Session      SessionDriverType
	Notification NotificationDriverType
}

func NewCellDrivers(source SourceDriverType, container ContainerDriverType, session SessionDriverType, notification NotificationDriverType) CellDrivers {
	return CellDrivers{Source: source, Container: container, Session: session, Notification: notification}
}

func CellResourceDrivers(cell Cell) CellDrivers {
	return NewCellDrivers(cell.Sources.Driver, cell.Containers.Driver, cell.Session.Driver, cell.NotificationDriver)
}

type CellRetrySpec struct {
	ID          string
	Issue       string
	Name        string
	Project     string
	Template    string
	Command     string
	FailedStage CreationStage
}

func NewCellRetrySpec(id string, issue string, name string, project string, templateName string, command string, failedStage CreationStage) CellRetrySpec {
	return CellRetrySpec{ID: id, Issue: issue, Name: name, Project: project, Template: templateName, Command: command, FailedStage: failedStage}
}

func InspectCellRetry(cell Cell) CellRetrySpec {
	return NewCellRetrySpec(cell.ID, cell.Issue, CellName(cell), cell.Project, cell.Template, cell.Creation.Command, cell.Creation.FailedStage)
}

func CellRetryAttemptMatches(cell Cell, attemptID string) bool {
	return CellCreationStatus(cell) == CreationRetrying && cell.Creation.AttemptID == attemptID
}

func PrepareCellRetryPersistence(target Cell, current Cell, attemptID string) (Cell, error) {
	if !CellRetryAttemptMatches(current, attemptID) {
		return Cell{}, fmt.Errorf("retry ownership lost for cell %q", CellName(target))
	}
	if CellCreationStatus(target) == CreationRetrying {
		target.Creation.LeaseStartedAt = current.Creation.LeaseStartedAt
		target.Creation.LeaseHeartbeatAt = current.Creation.LeaseHeartbeatAt
	}
	target.Version = current.Version
	return target, nil
}

func RefreshCellForRetry(stored Cell, rendered Cell) Cell {
	refreshed := rendered
	refreshed.ID = stored.ID
	refreshed.Issue = stored.Issue
	refreshed.Project = stored.Project
	refreshed.Note = stored.Note
	refreshed.Template = stored.Template
	refreshed.Creation = stored.Creation
	refreshed.Version = stored.Version
	refreshed.Status = stored.Status
	refreshed.Done = stored.Done
	refreshed.NotificationDriver = stored.NotificationDriver
	return refreshed
}

func BeginCellCreation(cell Cell, command string) Cell {
	cell.Creation = CellCreation{Status: CreationCreating, Command: command}
	return cell
}

func ResumeCellCreation(cell Cell) Cell {
	cell.Creation.Status = CreationCreating
	cell.Creation.FailedStage = ""
	cell.Creation.LastError = ""
	return cell
}

func BeginCellRetry(cell Cell, attemptID string, now time.Time) Cell {
	now = now.UTC()
	cell.Creation.Status = CreationRetrying
	cell.Creation.AttemptID = attemptID
	cell.Creation.LeaseStartedAt = &now
	cell.Creation.LeaseHeartbeatAt = &now
	return cell
}

func HeartbeatCellRetry(cell Cell, now time.Time) Cell {
	now = now.UTC()
	cell.Creation.LeaseHeartbeatAt = &now
	return cell
}

func CellRetryLeaseValid(cell Cell, now time.Time, timeout time.Duration) bool {
	return CellCreationStatus(cell) == CreationRetrying && cell.Creation.AttemptID != "" &&
		cell.Creation.LeaseHeartbeatAt != nil && !now.UTC().After(cell.Creation.LeaseHeartbeatAt.Add(timeout))
}

func CompleteCellCreationStage(cell Cell, stage CreationStage) Cell {
	if !CellCreationStageCompleted(cell, stage) {
		cell.Creation.CompletedStages = append(cell.Creation.CompletedStages, stage)
	}
	cell.Creation.FailedStage = ""
	cell.Creation.LastError = ""
	return cell
}

func ResetCellCreationStage(cell Cell, stage CreationStage) Cell {
	completed := make([]CreationStage, 0, len(cell.Creation.CompletedStages))
	for _, current := range cell.Creation.CompletedStages {
		if current != stage {
			completed = append(completed, current)
		}
	}
	cell.Creation.CompletedStages = completed
	return cell
}

func FailCellCreation(cell Cell, stage CreationStage, err error) Cell {
	cell.Creation.Status = CreationFailed
	cell.Creation.AttemptID = ""
	cell.Creation.LeaseStartedAt = nil
	cell.Creation.LeaseHeartbeatAt = nil
	cell.Creation.FailedStage = stage
	cell.Creation.LastError = ""
	if err != nil {
		cell.Creation.LastError = err.Error()
	}
	return cell
}

func FinishCellCreation(cell Cell) Cell {
	cell.Creation.Status = CreationReady
	cell.Creation.FailedStage = ""
	cell.Creation.LastError = ""
	cell.Creation.AttemptID = ""
	cell.Creation.LeaseStartedAt = nil
	cell.Creation.LeaseHeartbeatAt = nil
	return cell
}

func CellCreationStatus(cell Cell) CreationStatus {
	return cell.Creation.Status
}

func CellCreationStageCompleted(cell Cell, stage CreationStage) bool {
	for _, completed := range cell.Creation.CompletedStages {
		if completed == stage {
			return true
		}
	}
	return false
}

func NormalizeCellNote(note string) (string, error) {
	normalized := strings.Join(strings.FieldsFunc(note, unicode.IsSpace), " ")
	length := len([]rune(normalized))
	if length == 0 || length > 20 {
		return "", fmt.Errorf("cell note must be between 1 and 20 characters after whitespace normalization")
	}
	return normalized, nil
}

func SetCellNote(cell Cell, note string) (Cell, error) {
	normalized, err := NormalizeCellNote(note)
	if err != nil {
		return Cell{}, err
	}
	cell.Note = normalized
	return cell, nil
}

func MarkCellDone(cell Cell) (Cell, error) {
	if cell.Done {
		return Cell{}, fmt.Errorf("cell is already done")
	}
	cell.Done = true
	return cell, nil
}

func ToggleCellDone(cell Cell) Cell {
	cell.Done = !cell.Done
	return cell
}

func SetCellStatus(cell Cell, status CellStatus) (Cell, error) {
	validated, err := NewCellStatus(string(status))
	if err != nil {
		return Cell{}, err
	}
	cell.Status = validated
	return cell, nil
}

func EnsureCellCanBeCleaned(cell Cell) error {
	if !cell.Done {
		return fmt.Errorf("完了済みではないので消せない")
	}
	return nil
}

func ResolveCell(cells []Cell, identifier string) (Cell, bool) {
	for _, cell := range cells {
		if CellMatches(cell, identifier) {
			return cell, true
		}
	}
	return Cell{}, false
}

func CellMatches(cell Cell, identifier string) bool {
	return cell.ID == identifier || cell.Issue == identifier || CellName(cell) == identifier
}

func EnsureCellUnique(existing []Cell, issue string, name string) error {
	for _, cell := range existing {
		if cell.Issue == issue {
			return fmt.Errorf("cell issue %q already exists", issue)
		}
		if CellName(cell) == name {
			return fmt.Errorf("cell name %q already exists", name)
		}
	}
	return nil
}

func CellUsesDependency(cell Cell) bool {
	for _, container := range cell.Containers.Items {
		if container.Mode == Dependency {
			return true
		}
	}
	return false
}

func CellAfterPersistence(cell Cell) Cell {
	cell.Version++
	return cell
}

type StoredCell struct {
	Version            uint64       `json:"version"`
	ID                 string       `json:"id"`
	Issue              string       `json:"issue"`
	Project            string       `json:"project"`
	Note               string       `json:"note"`
	Template           string       `json:"template"`
	Sources            Sources      `json:"sources"`
	Containers         Containers   `json:"containers"`
	Session            Session      `json:"session"`
	NotificationDriver string       `json:"notificationDriver"`
	Creation           CellCreation `json:"creation"`
	Status             string       `json:"status"`
	Done               bool         `json:"done"`
}

func NewStoredCell(version uint64, id string, issue string, project string, note string, templateName string, sources Sources, containers Containers, session Session, notificationDriver string, creation CellCreation, status string, done bool) StoredCell {
	return StoredCell{Version: version, ID: id, Issue: issue, Project: project, Note: note, Template: templateName, Sources: sources, Containers: containers, Session: session, NotificationDriver: notificationDriver, Creation: creation, Status: status, Done: done}
}

func StoreCell(cell Cell) StoredCell {
	return NewStoredCell(uint64(cell.Version), cell.ID, cell.Issue, cell.Project, cell.Note, cell.Template, cell.Sources, cell.Containers, cell.Session, string(cell.NotificationDriver), cell.Creation, string(cell.Status), cell.Done)
}

func RestoreCell(stored StoredCell) (Cell, error) {
	version, err := NewCellVersion(stored.Version)
	if err != nil {
		return Cell{}, err
	}
	status, err := NewCellStatus(stored.Status)
	if err != nil {
		return Cell{}, err
	}
	creationStatus, err := NewCreationStatus(string(stored.Creation.Status))
	if err != nil {
		return Cell{}, err
	}
	stored.Creation.Status = creationStatus
	for _, stage := range stored.Creation.CompletedStages {
		if _, err := NewCreationStage(string(stage)); err != nil {
			return Cell{}, err
		}
	}
	if stored.Creation.FailedStage != "" {
		if _, err := NewCreationStage(string(stored.Creation.FailedStage)); err != nil {
			return Cell{}, err
		}
	}
	sourceDriver, err := NewSourceDriverType(string(stored.Sources.Driver))
	if err != nil {
		return Cell{}, err
	}
	containerDriver := stored.Containers.Driver
	switch stored.Containers.Driver {
	case None, Docker:
	default:
		return Cell{}, fmt.Errorf("invalid container driver type %q", stored.Containers.Driver)
	}
	sessionDriver, err := NewSessionDriverType(string(stored.Session.Driver))
	if err != nil {
		return Cell{}, err
	}
	sources := make([]Source, 0, len(stored.Sources.Items))
	for _, source := range stored.Sources.Items {
		validated, err := NewSource(source.Path, source.Base, source.Branch)
		if err != nil {
			return Cell{}, err
		}
		sources = append(sources, validated)
	}
	containers := make([]Container, 0, len(stored.Containers.Items))
	for _, container := range stored.Containers.Items {
		validated, err := NewContainer(container.Role, container.Network, container.SourceContainer, container.Mode)
		if err != nil {
			return Cell{}, err
		}
		containers = append(containers, validated)
	}
	windows := make([]SessionWindow, 0, len(stored.Session.Windows))
	for _, window := range stored.Session.Windows {
		validated, err := NewSessionWindow(window.Name, window.Command)
		if err != nil {
			return Cell{}, err
		}
		windows = append(windows, validated)
	}
	notificationDriver, err := NewNotificationDriverType(stored.NotificationDriver)
	if err != nil {
		return Cell{}, err
	}
	cell, err := NewCell(stored.ID, stored.Issue, stored.Project, stored.Template, NewSources(sourceDriver, sources), NewContainers(containerDriver, containers), NewSession(sessionDriver, windows), notificationDriver)
	if err != nil {
		return Cell{}, err
	}
	cell.Version, cell.Note, cell.Creation, cell.Status, cell.Done = version, stored.Note, stored.Creation, status, stored.Done
	return cell, nil
}

func CloneCell(cell Cell) Cell {
	cell.Sources.Items = append([]Source(nil), cell.Sources.Items...)
	cell.Containers.Items = append([]Container(nil), cell.Containers.Items...)
	for i := range cell.Containers.Items {
		cell.Containers.Items[i].Network = append([]string(nil), cell.Containers.Items[i].Network...)
	}
	cell.Session.Windows = append([]SessionWindow(nil), cell.Session.Windows...)
	cell.Creation.CompletedStages = append([]CreationStage(nil), cell.Creation.CompletedStages...)
	return cell
}
