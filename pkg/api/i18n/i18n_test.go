package i18n

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConvertLang(t *testing.T) {
	tests := []struct {
		name string
		want string
		lang string
	}{
		{
			name: "helloWorld",
			want: "Hello world!",
			lang: "en-EN,ru;q=0.5",
		},
		{
			name: "helloWorld",
			want: "Hello world!",
			lang: "en",
		},
		{
			name: "helloWorld",
			want: "Привет, мир!",
			lang: "ru-RU,ru;q=0.5",
		},
		{
			name: "helloWorld",
			want: "Привет, мир!",
			lang: "ru",
		},
		{
			name: "unknownName",
			want: "",
			lang: "ru-RU,ru;q=0.5",
		},
		{
			name: "helloWorld",
			want: "Hello world!",
			lang: "unknownLang", // default en
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := T(tt.lang, C{
				DefaultMessage: &M{
					ID: tt.name,
				},
			})
			require.Equal(t, tt.want, got)
		})
	}
}
