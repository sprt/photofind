package photofind

import (
	"encoding/base64"
	"io/ioutil"
	"mime/multipart"
	"net/http"

	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/vision/v1"
)

func annotateImages(ctx context.Context, images []*multipart.FileHeader) ([]*vision.AnnotateImageResponse, error) {
	client, err := google.DefaultClient(ctx, vision.CloudPlatformScope)
	if err != nil {
		return nil, err
	}

	vis, err := newGvision(ctx, client)
	if err != nil {
		return nil, err
	}

	return vis.annotate(images)
}

type gvision struct {
	service *vision.Service
}

func newGvision(ctx context.Context, client *http.Client) (*gvision, error) {
	service, err := vision.New(client)
	if err != nil {
		return nil, err
	}
	return &gvision{service: service}, nil
}

func (v *gvision) annotate(images []*multipart.FileHeader) ([]*vision.AnnotateImageResponse, error) {
	reqs := make([]*vision.AnnotateImageRequest, 0, len(images))
	for _, img := range images {
		req, err := makeRequest(img)
		if err != nil {
			return nil, err
		}
		reqs = append(reqs, req)
	}

	batch := &vision.BatchAnnotateImagesRequest{Requests: reqs}
	res, err := v.service.Images.Annotate(batch).Do()
	if err != nil {
		return nil, err
	}

	return res.Responses, nil
}

func makeRequest(image *multipart.FileHeader) (*vision.AnnotateImageRequest, error) {
	f, err := image.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()

	p, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	req := &vision.AnnotateImageRequest{
		Image: &vision.Image{
			Content: base64.StdEncoding.EncodeToString(p),
		},
		Features: []*vision.Feature{
			{Type: "TEXT_DETECTION"},
		},
	}
	return req, nil
}
