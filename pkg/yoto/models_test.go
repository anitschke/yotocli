package yoto

import "testing"

func TestAudioSHA256(t *testing.T) {
	const hash = "U-8tT6z7rt5MtIMzjANOtLmB0E7FamNOlQHBP1Vn4og"

	tests := []struct {
		name     string
		trackURL string
		want     string
	}{
		{
			// What a track is written as.
			name:     "a yoto reference",
			trackURL: "yoto:#" + hash,
			want:     hash,
		},
		{
			// What the same track is read back as. Missing this is what made a
			// sync reset every icon: nothing ever matched.
			name:     "a signed url",
			trackURL: "https://secure-media.yotoplay.com/prefix~/" + hash + "?Expires=1788994229&Signature=abc__#sha256=" + hash,
			want:     hash,
		},
		{
			name:     "a url with no hash",
			trackURL: "https://example.com/story.mp3",
			want:     "",
		},
		{
			name:     "nothing at all",
			trackURL: "",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AudioSHA256(tt.trackURL); got != tt.want {
				t.Errorf("AudioSHA256() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIconRef(t *testing.T) {
	const hash = "aUm9i3ex3qqAMYBv-i-O-pYMKuMJGICtR3Vhf289u2Q"

	tests := []struct {
		name string
		icon string
		want string
	}{
		{
			name: "a url as the api returns it",
			icon: "https://card-content.yotoplay.com/prefix~/" + hash,
			want: "yoto:#" + hash,
		},
		{
			name: "a url with query parameters",
			icon: "https://card-content.yotoplay.com/prefix~/" + hash + "?size=16",
			want: "yoto:#" + hash,
		},
		{
			name: "a reference is already right",
			icon: "yoto:#" + hash,
			want: "yoto:#" + hash,
		},
		{
			// Not something we can make sense of, so it goes as it came and the
			// API can rule on it.
			name: "a url that does not end in a hash",
			icon: "https://example.com/icon.png",
			want: "https://example.com/icon.png",
		},
		{
			name: "no icon",
			icon: "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IconRef(tt.icon); got != tt.want {
				t.Errorf("IconRef() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseMediaID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"yoto:#7Wt--qhx9t7PCSC8WBmGKJvlG53Z1uk7bN1dE9wGWJ0", "7Wt--qhx9t7PCSC8WBmGKJvlG53Z1uk7bN1dE9wGWJ0"},
		{"https://media-secure-v2-test.aws.fooropa.com/icons/7Wt--qhx9t7PCSC8WBmGKJvlG53Z1uk7bN1dE9wGWJ0", "7Wt--qhx9t7PCSC8WBmGKJvlG53Z1uk7bN1dE9wGWJ0"},
		{"https://media-secure-v2-test.aws.fooropa.com/icons/7Wt--qhx9t7PCSC8WBmGKJvlG53Z1uk7bN1dE9wGWJ0?foo=bar", "7Wt--qhx9t7PCSC8WBmGKJvlG53Z1uk7bN1dE9wGWJ0"},
		{"7Wt--qhx9t7PCSC8WBmGKJvlG53Z1uk7bN1dE9wGWJ0", "7Wt--qhx9t7PCSC8WBmGKJvlG53Z1uk7bN1dE9wGWJ0"},
	}
	for _, tt := range tests {
		if got := ParseMediaID(tt.input); got != tt.want {
			t.Errorf("ParseMediaID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
