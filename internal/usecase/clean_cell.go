package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/hgsg11/paracell/internal/domain"
)

type CleanCellInput struct {
	Cell string
}

type CleanCellUseCase struct {
	State            CellStatePort
	SourceFactory    SourceProviderFactory
	ContainerFactory ContainerProviderFactory
	SessionFactory   SessionProviderFactory
}

func (u CleanCellUseCase) Execute(ctx context.Context, input CleanCellInput) error {
	cells, err := u.State.LoadCells(ctx)
	if err != nil {
		return err
	}
	target, ok := domain.ResolveCell(cells, input.Cell)
	if !ok {
		return fmt.Errorf("cell %q not found", input.Cell)
	}
	if err := target.EnsureCanBeCleaned(); err != nil {
		return err
	}
	drivers := target.ResourceDrivers()
	session, err := u.SessionFactory.Session(drivers.Session)
	if err != nil {
		return err
	}
	containers, err := u.ContainerFactory.Container(drivers.Container)
	if err != nil {
		return err
	}
	source, err := u.SourceFactory.Source(drivers.Source)
	if err != nil {
		return err
	}
	if err := ignoreNotFound(domain.CleanSession(ctx, target, session)); err != nil {
		return err
	}
	if err := ignoreNotFound(domain.CleanContainers(ctx, target, containers)); err != nil {
		return err
	}
	if err := ignoreNotFound(domain.CleanSources(ctx, target, source)); err != nil {
		return err
	}
	return u.State.DeleteCell(ctx, target)
}

func ignoreNotFound(err error) error {
	if err == nil {
		return nil
	}
	type multiUnwrapper interface {
		Unwrap() []error
	}
	if joined, ok := err.(multiUnwrapper); ok {
		var remaining error
		for _, nested := range joined.Unwrap() {
			remaining = errors.Join(remaining, ignoreNotFound(nested))
		}
		return remaining
	}
	type singleUnwrapper interface {
		Unwrap() error
	}
	if wrapped, ok := err.(singleUnwrapper); ok && errors.Is(wrapped.Unwrap(), domain.ErrNotFound) {
		return ignoreNotFound(wrapped.Unwrap())
	}
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	return err
}
