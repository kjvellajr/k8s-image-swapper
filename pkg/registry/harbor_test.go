package registry

import (
	"testing"

	"github.com/containers/image/v5/transports/alltransports"

	"github.com/stretchr/testify/assert"
)

func TestHarborIsOrigin(t *testing.T) {
	type testCase struct {
		input    string
		expected bool
	}
	testcases := []testCase{
		{
			input:    "k8s.gcr.io/ingress-nginx/controller@sha256:9bba603b99bf25f6d117cf1235b6598c16033ad027b143c90fa5b3cc583c5713",
			expected: false,
		},
		{
			input:    "harbor.example.com/docker-cache-foo/k8s.gcr.io/ingress-nginx/controller@sha256:9bba603b99bf25f6d117cf1235b6598c16033ad027b143c90fa5b3cc583c5713",
			expected: false,
		},
		{
			input:    "harbor.example.com/docker-cache/k8s.gcr.io/ingress-nginx/controller@sha256:9bba603b99bf25f6d117cf1235b6598c16033ad027b143c90fa5b3cc583c5713",
			expected: true,
		},
	}

	fakeRegistry := NewDummyHarborClient("harbor.example.com", "docker-cache")

	for _, testcase := range testcases {
		imageRef, err := alltransports.ParseImageName("docker://" + testcase.input)

		assert.NoError(t, err)

		result := fakeRegistry.IsOrigin(imageRef)

		assert.Equal(t, testcase.expected, result)
	}
}
