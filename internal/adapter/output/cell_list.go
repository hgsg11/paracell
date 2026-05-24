package output

import (
	"strings"

	"github.com/shige1114/paradev/internal/domain"
)

func FormatCellList(cells []domain.Cell) string {
	var b strings.Builder
	b.WriteString("NAME\tTEMPLATE\n")
	for _, cell := range cells {
		b.WriteString(cell.Name)
		b.WriteByte('\t')
		b.WriteString(cell.Template)
		b.WriteByte('\n')
	}
	return b.String()
}
