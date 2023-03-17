package api

type previewGenerator interface {
	GenerateImageUrl(url string, height, width int) string
}
