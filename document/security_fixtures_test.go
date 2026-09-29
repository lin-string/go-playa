package document

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/lin-string/go-playa/internal/testfixture"
)

func TestSecurityPDFFixturesDoNotPanicOrExpand(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join(testfixture.Dir(t), "security_*.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no security PDF fixtures found")
	}

	for _, path := range paths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("security fixture caused panic: %v", recovered)
				}
			}()

			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			base := filepath.Base(path)
			if base == "security_r5_aes256.pdf" || base == "security_r6_aes256.pdf" {
				d, err := OpenBytes(data, WithPassword("secret"))
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = d.Close() }()
				page, err := d.PageAt(0)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := d.PageText(page); err != nil {
					t.Fatal(err)
				}
				if _, err := d.MetadataXML(); err != nil {
					t.Fatal(err)
				}
				return
			}
			if base == "security_metadata_encrypt_cycle_overflow.pdf" || base == "security_metadata_encrypt_type_overflow.pdf" {
				_, err := OpenBytes(data, WithPassword("secret"))
				if err != ErrUnsupportedEncryption {
					t.Fatalf("cyclic EncryptMetadata error = %v, want %v", err, ErrUnsupportedEncryption)
				}
				return
			}
			d, err := OpenBytes(data)
			if err != nil {
				t.Fatal(err)
			}
			var pages []Page
			for page, pageErr := range d.Pages() {
				if pageErr != nil {
					t.Fatal(pageErr)
				}
				pages = append(pages, page)
			}
			if len(pages) != 1 {
				t.Fatalf("pages = %d, want 1", len(pages))
			}
			if _, err := d.PageText(pages[0]); err != nil {
				t.Logf("page text rejected malformed input: %v", err)
			}
			for image, imageErr := range d.ImagesSeq(pages[0]) {
				if imageErr != nil {
					t.Logf("image rejected malformed input: %v", imageErr)
					continue
				}
				_, _ = image.Samples()
			}

			if filepath.Base(path) == "security_metadata_filter_overflow.pdf" {
				if _, err := d.MetadataXML(); err == nil {
					t.Fatal("unknown metadata filter was accepted")
				}
			}
			if filepath.Base(path) == "security_metadata_cycle_overflow.pdf" {
				d.recordMetadataRef()
				if len(d.metadataRefs) != 2 {
					t.Fatalf("metadata cycle references = %#v, want two visited references", d.metadataRefs)
				}
			}
		})
	}
}

func TestSecurityPDFFixtureNamesRemainStable(t *testing.T) {
	want := []string{
		"security_cff_index_overflow.pdf",
		"security_cid_width_overflow.pdf",
		"security_cmap_range_overflow.pdf",
		"security_icc_components_overflow.pdf",
		"security_image_geometry_overflow.pdf",
		"security_metadata_cycle_overflow.pdf",
		"security_metadata_encrypt_cycle_overflow.pdf",
		"security_metadata_encrypt_type_overflow.pdf",
		"security_metadata_filter_overflow.pdf",
		"security_object_stream_offset_overflow.pdf",
		"security_r5_aes256.pdf",
		"security_r6_aes256.pdf",
		"security_stream_length_overflow.pdf",
		"security_tounicode_overflow.pdf",
		"security_truetype_cmap_overflow.pdf",
	}
	got, err := filepath.Glob(filepath.Join(testfixture.Dir(t), "security_*.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	for i := range got {
		got[i] = filepath.Base(got[i])
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("security fixtures = %v, want %v", got, want)
	}
}

func TestModernSecurityPDFFixturesDecryptPage(t *testing.T) {
	fixtures := []struct {
		name string
		text string
	}{
		{name: "security_r5_aes256.pdf", text: "Modern R5 encrypted report"},
		{name: "security_r6_aes256.pdf", text: "Modern R6 encrypted report"},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			data, err := os.ReadFile(testfixture.Path(t, fixture.name))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := OpenBytes(data, WithPassword("wrong")); err != ErrInvalidPassword {
				t.Fatalf("wrong password error = %v, want %v", err, ErrInvalidPassword)
			}
			d, err := OpenBytes(data, WithPassword("secret"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = d.Close() }()
			page, err := d.PageAt(0)
			if err != nil {
				t.Fatal(err)
			}
			texts, err := d.PageText(page)
			if err != nil || len(texts) != 1 || texts[0].Text() != fixture.text {
				t.Fatalf("decrypted text = %#v, err=%v", texts, err)
			}
			metadata, err := d.MetadataXML()
			if err != nil || string(metadata) != "<x:xmpmeta><dc:title>Modern encrypted report</dc:title></x:xmpmeta>" {
				t.Fatalf("excluded metadata = %q, err=%v", metadata, err)
			}
		})
	}
}

func TestSecurityToUnicodeOverflowFallsBackToUnknownByteFont(t *testing.T) {
	data, err := os.ReadFile(testfixture.Path(t, "security_tounicode_overflow.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	page, pageErr := d.PageAt(0)
	if pageErr != nil {
		t.Fatal(pageErr)
	}
	texts, err := d.PageText(page)
	if err != nil {
		t.Fatal(err)
	}
	if len(texts) != 1 || texts[0].Text() != string([]byte{0, 1}) || len(texts[0].glyphs) != 2 {
		t.Fatalf("overflow ToUnicode text = %#v", texts)
	}
	for font, fontErr := range d.PageFontSeq(page) {
		if fontErr != nil {
			t.Fatal(fontErr)
		}
		metadata, ok := font.Metadata()
		if !ok || metadata.Name() != "unknown" {
			t.Fatalf("overflow ToUnicode font metadata = %#v, ok=%v", metadata, ok)
		}
		return
	}
	t.Fatal("overflow ToUnicode page had no fonts")
}

func TestMalformedFontMappingsUsePlayaDummyFontMetadata(t *testing.T) {
	fixtures := []string{
		"malicious_cmap.pdf",
		"security_cmap_range_overflow.pdf",
		"security_tounicode_overflow.pdf",
	}
	for _, fixture := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			data, err := os.ReadFile(testfixture.Path(t, fixture))
			if err != nil {
				t.Fatal(err)
			}
			d, err := OpenBytes(data)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = d.Close() }()
			page, err := d.PageAt(0)
			if err != nil {
				t.Fatal(err)
			}

			for resource, resourceErr := range page.FontsSeq(d) {
				if resourceErr != nil {
					t.Fatal(resourceErr)
				}
				font := resource.FontCopy()
				metadata, ok := resource.Metadata()
				if !ok || metadata.Name() != "unknown" {
					t.Fatalf("font metadata = %#v, ok=%v; want Playa dummy metadata", metadata, ok)
				}
				if font == nil || font.Name() != "unknown" || font.BaseFont() != "unknown" || font.CIDCoding() != "" || font.IsCID() || font.IsVertical() {
					t.Fatalf("font fallback = %#v; want unknown non-CID non-vertical font", font)
				}
				return
			}
			t.Fatal("malformed font fixture had no page fonts")
		})
	}
}
