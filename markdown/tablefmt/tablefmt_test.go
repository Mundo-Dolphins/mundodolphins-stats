package tablefmt

import "testing"

func TestRenderMarkdownTable(t *testing.T) {
	table := Table{
		Headers: []string{"Player", "Note"},
		Rows: [][]string{
			{"Tua", "QB"},
			{"Hill", "WR"},
		},
	}

	out, err := RenderMarkdownTable(table)
	if err != nil {
		t.Fatalf("RenderMarkdownTable error: %v", err)
	}

	expected := "| Player | Note |\n| ------ | ---- |\n| Tua    | QB   |\n| Hill   | WR   |"
	if out != expected {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestRenderMarkdownTable_EscapesPipes(t *testing.T) {
	table := Table{
		Headers: []string{"Col"},
		Rows: [][]string{
			{"A|B"},
		},
	}

	out, err := RenderMarkdownTable(table)
	if err != nil {
		t.Fatalf("RenderMarkdownTable error: %v", err)
	}
	if out != "| Col  |\n| ---- |\n| A\\|B |" {
		t.Errorf("unexpected output: %s", out)
	}
}

func TestRenderMarkdownTable_EmptyHeaders(t *testing.T) {
	if _, err := RenderMarkdownTable(Table{}); err == nil {
		t.Fatalf("expected error for empty headers")
	}
}

func TestRenderMarkdownTable_BadRowWidth(t *testing.T) {
	table := Table{Headers: []string{"A"}, Rows: [][]string{{"1", "2"}}}
	if _, err := RenderMarkdownTable(table); err == nil {
		t.Fatalf("expected error for mismatched row width")
	}
}
