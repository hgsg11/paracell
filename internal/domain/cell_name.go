package domain

type CellName struct {
	Value string
}

func NewCellName(issue string) CellName {
	return CellName{Value: SafeResourceName(issue, "cell")}
}
