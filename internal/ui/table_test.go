package ui

import "testing"

func TestRenderTableUsesOneHeaderRuleAndUnboxedRows(t *testing.T) {
	got := RenderTable(
		[]string{"Projeto", "Aprovação", "Pasta"},
		[][]string{
			{"konen", "aprovado", "~/code/konen"},
			{"sample-app", "revisão necessária", "~/code/sample-app"},
		},
	)
	want := "Projeto    | Aprovação          | Pasta\n" +
		"───────────────────────────────────────────────────\n" +
		"konen        aprovado             ~/code/konen\n" +
		"sample-app   revisão necessária   ~/code/sample-app\n"
	if got != want {
		t.Fatalf("RenderTable() =\n%s\nwant:\n%s", got, want)
	}
}
