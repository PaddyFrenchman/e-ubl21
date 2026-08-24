package xmlnode

import "testing"

func TestRenderEscapesAndSortsAttributes(t *testing.T) {
	got := Render(Node{
		Prefix: "cac",
		Name:   "Parent",
		Attributes: map[string]string{
			"z": `a&b`,
			"a": `"quoted"`,
		},
		Children: []Node{{Name: "Child", Value: `<value>`}},
	}, 1)
	want := "  <cac:Parent a=\"&quot;quoted&quot;\" z=\"a&amp;b\">\n" +
		"      <Child>&lt;value&gt;</Child>\n" +
		"  </cac:Parent>\n"
	if got != want {
		t.Fatalf("Render() = %q, want %q", got, want)
	}
	if got := indentString(0); got != "" {
		t.Fatalf("indentString(0) = %q", got)
	}
}
