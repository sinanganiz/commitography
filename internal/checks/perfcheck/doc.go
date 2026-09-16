// Package perfcheck measures the local server's performance and resource use.
// The measurements are compiled only with the perfcheck build tag:
//
//	go test -tags perfcheck -count=1 -v -timeout 60m ./internal/checks/perfcheck
//
// They generate their own large repositories, so the numbers can be
// reproduced from a checkout. COMMITOGRAPHY_PERF_REPO adds a real repository
// to the analysis timings. The Docker measurements run only when Docker is
// available. Each test logs its numbers and fails only when a recorded bound is
// exceeded.
package perfcheck
