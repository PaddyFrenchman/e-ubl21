package xmlnode

import (
	"strings"
	"testing"
)

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
	var streamed strings.Builder
	RenderTo(&streamed, Node{Name: "Child", Value: "value"}, 0)
	if got := streamed.String(); got != "<Child>value</Child>\n" {
		t.Fatalf("RenderTo() = %q", got)
	}
}
