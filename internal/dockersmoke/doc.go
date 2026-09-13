// Package dockersmoke holds the Docker smoke tests for the container image.
// They are compiled only with the dockersmoke build tag:
//
//	go test -tags dockersmoke -count=1 -v ./internal/dockersmoke
//
// The tests build the image from this checkout, or test COMMITOGRAPHY_IMAGE
// when it names an existing image, and skip when Docker or the generated
// fixtures are unavailable.
package dockersmoke
