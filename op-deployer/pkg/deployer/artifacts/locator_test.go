package artifacts

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLocator_Marshaling(t *testing.T) {
	tests := []struct {
		name string
		in   string
		out  *Locator
		err  bool
	}{
		{
			name: "valid HTTPS URL",
			in:   "https://example.com",
			out: &Locator{
				URL: parseUrl(t, "https://example.com"),
			},
			err: false,
		},
		{
			name: "valid HTTP URL",
			in:   "http://example.com",
			out: &Locator{
				URL: parseUrl(t, "http://example.com"),
			},
			err: false,
		},
		{
			name: "valid file URL",
			in:   "file:///tmp/artifacts",
			out: &Locator{
				URL: parseUrl(t, "file:///tmp/artifacts"),
			},
			err: false,
		},
		{
			name: "empty",
			in:   "",
			out:  nil,
			err:  true,
		},
		{
			name: "no scheme",
			in:   "example.com",
			out:  nil,
			err:  true,
		},
		{
			name: "unsupported scheme",
			in:   "ftp://example.com",
			out:  nil,
			err:  true,
		},
		{
			name: "embedded",
			in:   "embedded",
			out: &Locator{
				URL: embeddedURL,
			},
			err: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var a Locator
			err := a.UnmarshalText([]byte(tt.in))
			if tt.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.out, &a)

			marshalled, err := a.MarshalText()
			require.NoError(t, err)
			require.Equal(t, tt.in, string(marshalled))
		})
	}
}

func parseUrl(t *testing.T, u string) *url.URL {
	parsed, err := url.Parse(u)
	require.NoError(t, err)
	return parsed
}

func TestLocator_Equal(t *testing.T) {
	tests := []struct {
		a     *Locator
		b     *Locator
		equal bool
	}{
		{
			MustNewLocatorFromURL("https://www.example.com"),
			MustNewLocatorFromURL("http://www.example.com"),
			false,
		},
		{
			MustNewLocatorFromURL("http://www.example.com"),
			MustNewLocatorFromURL("http://www.example.com"),
			true,
		},
		{
			MustNewFileLocator("/foo/bar"),
			MustNewFileLocator("/foo/bar"),
			true,
		},
		{
			MustNewFileLocator("/foo/bar"),
			MustNewFileLocator("/foo/baz"),
			false,
		},
	}
	for _, test := range tests {
		if test.equal {
			require.True(t, test.a.Equal(test.b), "%s != %s", test.a, test.b)
		} else {
			require.False(t, test.a.Equal(test.b), "%s == %s", test.a, test.b)
		}
	}
}
