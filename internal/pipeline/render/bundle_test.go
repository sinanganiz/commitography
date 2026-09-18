package render

import (
	"regexp"
	"strings"
	"testing"
)

func TestEmbeddedBundleReferencesNoExternalResources(t *testing.T) {
	t.Parallel()
	// allowedURLPrefixes are the absolute URLs the bundled libraries carry as
	// plain strings: XML namespaces that name SVG and MathML elements, and the
	// documentation pages React and MUI link from their production error
	// messages. None of them is fetched. A new URL fails this test until
	// someone confirms it is not a remote resource and adds it here.
	allowedURLPrefixes := []string{
		"http://www.w3.org/",
		"https://react.dev/errors/",
		"https://mui.com/production-error/",
	}
	absoluteURL := regexp.MustCompile(`https?://[^\s"'` + "`" + `)<>\\]*`)

	for _, name := range []string{"assets/app.js", "assets/app.css"} {
		data, err := assets.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)

		for _, url := range absoluteURL.FindAllString(source, -1) {
			allowed := false
			for _, prefix := range allowedURLPrefixes {
				if strings.HasPrefix(url, prefix) {
					allowed = true
					break
				}
			}
			if !allowed {
				t.Errorf("%s contains an unreviewed external URL %q", name, url)
			}
		}

		// Loading a remote resource needs one of these forms; none may point
		// at an absolute or protocol-relative URL.
		for _, pattern := range []string{
			`import\(\s*["'](https?:)?//`,
			`fetch\(\s*["'](https?:)?//`,
			`(src|href)\s*=\s*["'](https?:)?//`,
			`url\(\s*["']?(https?:)?//`,
			`@import`,
			`importScripts\(`,
			`new\s+WebSocket\(`,
			`new\s+EventSource\(`,
		} {
			if name == "assets/app.js" && pattern == `@import` {
				// The bundled CSS-in-JS parser names @import as a string
				// constant; the stylesheet check below is the one that matters.
				continue
			}
			if match := regexp.MustCompile(pattern).FindString(source); match != "" {
				t.Errorf("%s loads a remote resource: %q", name, match)
			}
		}
	}
}
