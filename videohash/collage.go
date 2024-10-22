package videohash

import (
	"fmt"
	"math"
	"os"

	"image"
	"image/png"

	"golang.org/x/image/draw"
)

func getImage(imagePath string) (image.Image, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, fmt.Errorf("unable to open image: %w", err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("unable to decode image: %w", err)
	}

	return img, err
}

func saveImage(imagePath string, img image.Image) error {
	file, err := os.Create(imagePath)
	if err != nil {
		return fmt.Errorf("unable to open image: %w", err)
	}
	defer file.Close()

	err = png.Encode(file, img)
	if err != nil {
		return fmt.Errorf("unable to decode image: %w", err)
	}

	return err
}

func createCollage(framesFilenames []string, collagePath string) error {
	if len(framesFilenames) == 0 {
		return fmt.Errorf("no frames")
	}

	frame0, err := getImage(framesFilenames[0])
	if err != nil {
		return fmt.Errorf("unable to open first frame: %w", err)
	}
	frameImageWidth, frameImageHeight := frame0.Bounds().Size().X, frame0.Bounds().Size().Y

	collageImageWidth := 1024
	imagesPerRowInCollage := int(math.Ceil(math.Sqrt(float64(len(framesFilenames)))))

	scale := float64(collageImageWidth) / (float64(imagesPerRowInCollage) * float64(frameImageWidth))

	scaledFrameImageWidth := int(math.Ceil(float64(frameImageWidth) * scale))
	scaledFrameImageHeight := int(math.Ceil(float64(frameImageHeight) * scale))

	numberOfRows := math.Ceil(float64(len(framesFilenames)) / float64(imagesPerRowInCollage))

	collageImageHeight := int(math.Round(scale * float64(frameImageHeight) * numberOfRows))

	collageImage := image.NewRGBA(image.Rect(0, 0, collageImageWidth, collageImageHeight))

	i, j := 0, 0

	for count, framePath := range framesFilenames {
		if count%imagesPerRowInCollage == 0 {
			i = 0
		}

		frame, err := getImage(framePath)
		if err != nil {
			return fmt.Errorf("unbale to open frame: %w", err)
		}

		frameRescaled := image.NewRGBA(image.Rect(0, 0, scaledFrameImageWidth, scaledFrameImageHeight))

		draw.NearestNeighbor.Scale(frameRescaled, frameRescaled.Rect, frame, frame.Bounds(), draw.Over, nil)

		x := i

		y := int(j / imagesPerRowInCollage * scaledFrameImageHeight)

		draw.Over.Draw(
			collageImage,
			image.Rect(x, y, x+scaledFrameImageWidth, y+scaledFrameImageHeight),
			frameRescaled,
			frameRescaled.Rect.Min,
		)

		i += scaledFrameImageWidth

		j += 1
	}

	err = saveImage(collagePath, collageImage)
	if err != nil {
		return fmt.Errorf("unable to save collage: %w", err)
	}

	return nil
}
