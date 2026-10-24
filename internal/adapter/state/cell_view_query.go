package state

import (
	"context"

	"github.com/hgsg11/paracell/internal/domain"
	"github.com/hgsg11/paracell/internal/query"
)

var _ query.CellViewQuery = SQLiteCellAdapter{}

// ListCellViews reads the Cell list in its persisted order directly from SQLite,
// for every record format, without restoring the domain aggregates.
func (a SQLiteCellAdapter) ListCellViews(ctx context.Context) ([]query.CellView, error) {
	db, err := a.open(ctx)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := queryRows(ctx, db)
	if err != nil {
		return nil, err
	}
	views := make([]query.CellView, 0, len(rows))
	for _, row := range rows {
		views = append(views, newCellView(row.Record))
	}
	return views, nil
}

func newCellView(record cellRecord) query.CellView {
	group := record.CellGroup
	name := domain.NewCellName(group.Issue).Value
	label := group.Note
	if label == "" {
		label = name
	}
	view := query.CellView{
		Name: name, Label: label, Template: group.Template,
		Status: record.Commander.Status, Done: record.Commander.Done,
		CreationStatus: group.Creation.Status,
	}
	if group.Creation.Status == string(domain.CreationFailed) {
		view.FailedStage, view.LastError = group.Creation.FailedStage, group.Creation.LastError
	}
	return view
}
