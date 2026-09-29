package document_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/lin-string/go-playa/document"
)

// Two Forms share a resource graph containing both Forms, as in LibreOffice
// output. The page executes A once; neither Form executes another Form.
func resourceGraphPDF(invalid, indirectResources bool) []byte {
	properties := "<< /P << /MCID 7 >> >>"
	if invalid {
		properties = "42"
	}
	content := "/First MP /A Do /Span /P DP"
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 100] /Resources 5 0 R /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
		"<< /XObject << /A 6 0 R /B 7 0 R >> /Properties << /P << /MCID 7 >> >> >>",
		"<< /Type /XObject /Subtype /Form /BBox [0 0 10 10] /Resources 5 0 R /Length 0 >>\nstream\n\nendstream",
		"<< /Type /XObject /Subtype /Form /BBox [0 0 10 10] /Resources 8 0 R /Length 0 >>\nstream\n\nendstream",
		"<< /XObject << /A 9 0 R /B 7 0 R >> /Properties " + properties + " >>",
		"6 0 R",
	}
	if !indirectResources {
		for _, index := range []int{2, 5, 6} {
			objects[index] = strings.ReplaceAll(objects[index], "/Resources 5 0 R", "/Resources "+objects[4])
			objects[index] = strings.ReplaceAll(objects[index], "/Resources 8 0 R", "/Resources "+objects[7])
		}
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.7\n")
	offsets := make([]int, len(objects)+1)
	for i, obj := range objects {
		offsets[i+1] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&b, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	return b.Bytes()
}

func TestPublicTagsSharedCyclicResources(t *testing.T) {
	// Isolate the regression so an exponential traversal cannot hang the suite.
	const childEnv = "GO_PLAYA_RESOURCE_GRAPH_TEST"
	if os.Getenv(childEnv) != "1" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.CommandContext(ctx, executable, "-test.run=^TestPublicTagsSharedCyclicResources$")
		cmd.Env = append(os.Environ(), childEnv+"=1")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("shared resource traversal failed: %v (timeout: %v)\n%s", err, ctx.Err(), output)
		}
		return
	}
	for _, indirect := range []bool{false, true} {
		for _, invalid := range []bool{false, true} {
			t.Run(fmt.Sprintf("indirect=%t/invalid=%t", indirect, invalid), func(t *testing.T) {
				d, err := document.OpenBytes(resourceGraphPDF(invalid, indirect))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := d.Close(); err != nil {
						t.Error(err)
					}
				})
				for p, err := range d.Pages() {
					if err != nil {
						t.Fatal(err)
					}
					seq := d.PageTagsSeq(p)
					for tag, err := range seq {
						if err != nil || tag.Name() != "First" {
							t.Fatalf("early tag: %q, %v", tag.Name(), err)
						}
						break
					}
					for attempt := 0; attempt < 2; attempt++ {
						count, failures := 0, 0
						for tag, err := range seq {
							if err != nil {
								if !invalid || !strings.Contains(err.Error(), "Properties") {
									t.Fatalf("unexpected error: %v", err)
								}
								failures++
								continue
							}
							if count == 0 && tag.Name() != "First" {
								t.Fatalf("first tag: %q", tag.Name())
							}
							if count == 1 && (tag.Name() != "Span" || !tag.HasMCID() || tag.MCID() != 7) {
								t.Fatalf("property tag: %v", tag)
							}
							count++
						}
						if invalid {
							if count != 1 || failures != 1 {
								t.Fatalf("deferred validation: tags=%d errors=%d", count, failures)
							}
						} else if count != 2 || failures != 0 {
							t.Fatalf("repeat traversal: tags=%d errors=%d", count, failures)
						}
					}
				}
			})
		}
	}
}
