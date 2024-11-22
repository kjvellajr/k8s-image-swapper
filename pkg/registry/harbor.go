package registry

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"time"

	ctypes "github.com/containers/image/v5/types"
	"github.com/dgraph-io/ristretto"
	"github.com/estahn/k8s-image-swapper/pkg/config"
	"github.com/go-co-op/gocron"
	"github.com/rs/zerolog/log"
)

type HarborClient struct {
	harborURL string
	project   string
	authToken []byte
	cache     *ristretto.Cache
	scheduler *gocron.Scheduler
}

func NewHarborClient(clientConfig config.Harbor) (*HarborClient, error) {
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,     // number of keys to track frequency of (10M).
		MaxCost:     1 << 30, // maximum cost of cache (1GB).
		BufferItems: 64,      // number of keys per Get buffer.
	})
	if err != nil {
		panic(err)
	}

	scheduler := gocron.NewScheduler(time.UTC)
	scheduler.StartAsync()

	// token used to access Harbor server
	// example token: robot$<robot-account-name>:<robot-account-secret>
	harborAuthToken := os.Getenv("HARBOR_AUTH_TOKEN")

	client := &HarborClient{
		harborURL: clientConfig.URL,
		project:   clientConfig.Project,
		authToken: []byte(harborAuthToken),
		cache:     cache,
		scheduler: scheduler,
	}

	return client, nil
}

func (e *HarborClient) Credentials() string {
	return string(e.authToken)
}

// Repositories are not pre-created in Harbor
func (e *HarborClient) CreateRepository(ctx context.Context, name string) error {
	return nil
}

func (e *HarborClient) RepositoryExists() bool {
	panic("implement me")
}

func (e *HarborClient) CopyImage(ctx context.Context, srcRef ctypes.ImageReference, srcCreds string, destRef ctypes.ImageReference, destCreds string) error {
	src := srcRef.DockerReference().String()
	dest := destRef.DockerReference().String()
	app := "skopeo"
	args := []string{
		"--override-os", "linux",
		"copy",
		"--multi-arch", "all",
		"--retry-times", "3",
		"docker://" + src,
		"docker://" + dest,
	}

	if len(srcCreds) > 0 {
		args = append(args, "--src-authfile", srcCreds)
	} else {
		args = append(args, "--src-no-creds")
	}

	if len(destCreds) > 0 {
		args = append(args, "--dest-creds", destCreds)
	} else if len(e.Credentials()) > 0 {
		args = append(args, "--dest-creds", e.Credentials())
	} else {
		args = append(args, "--dest-no-creds")
	}

	log.Ctx(ctx).
		Trace().
		Str("app", app).
		Strs("args", args).
		Msg("execute command to copy image")

	output, cmdErr := exec.CommandContext(ctx, app, args...).CombinedOutput()

	// check if the command timed out during execution for proper logging
	if err := ctx.Err(); err != nil {
		return err
	}

	// enrich error with output from the command which may contain the actual reason
	if cmdErr != nil {
		return fmt.Errorf("Command error, stderr: %s, stdout: %s", cmdErr.Error(), string(output))
	}

	return nil
}

func (e *HarborClient) PullImage() error {
	panic("implement me")
}

func (e *HarborClient) PutImage() error {
	panic("implement me")
}

func (e *HarborClient) ImageExists(ctx context.Context, imageRef ctypes.ImageReference) bool {
	ref := imageRef.DockerReference().String()
	if _, found := e.cache.Get(ref); found {
		log.Ctx(ctx).Trace().Str("ref", ref).Msg("found in cache")
		return true
	}

	app := "skopeo"
	args := []string{
		"inspect",
		"--retry-times", "3",
		"docker://" + ref,
		"--creds", e.Credentials(),
	}

	log.Ctx(ctx).Trace().Str("app", app).Strs("args", args).Msg("executing command to inspect image")
	if err := exec.CommandContext(ctx, app, args...).Run(); err != nil {
		log.Ctx(ctx).Trace().Str("ref", ref).Msg("not found in target repository")
		return false
	}

	log.Ctx(ctx).Trace().Str("ref", ref).Msg("found in target repository")

	e.cache.SetWithTTL(ref, "", 1, 24*time.Hour+time.Duration(rand.Intn(180))*time.Minute)

	return true
}

func (e *HarborClient) Endpoint() string {
	return fmt.Sprintf("%s/%s", e.harborURL, e.project)
}

// IsOrigin returns true if the references origin is from this registry
func (e *HarborClient) IsOrigin(imageRef ctypes.ImageReference) bool {
	return strings.HasPrefix(imageRef.DockerReference().String(), e.Endpoint()+"/")
}

// For testing purposes
func NewDummyHarborClient(url string, project string) *HarborClient {
	return &HarborClient{
		harborURL: url,
		project:   project,
		authToken: []byte("mock-harbor-client-username:mock-harbor-client-password"),
	}
}
