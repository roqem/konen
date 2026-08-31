package ui

import "testing"

func TestRenderTableUsesOneHeaderRuleAndUnboxedRows(t *testing.T) {
	got := RenderTable(
		[]string{"PROJETO", "APROVAÇÃO", "PASTA"},
		[][]string{
			{"konen", "aprovado", "~/code/konen"},
			{"sample-app", "revisão necessária", "~/code/sample-app"},
		},
	)
	want := "PROJETO  | APROVAÇÃO          | PASTA\n" +
		"───────────────────────────────────────────────\n" +
		"konen      aprovado             ~/code/konen\n" +
		"sample-app   revisão necessária   ~/code/sample-app\n"
	if got != want {
		t.Fatalf("RenderTable() =\n%s\nwant:\n%s", got, want)
	}
}
