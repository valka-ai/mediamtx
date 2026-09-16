package core

import (
	"regexp"
	"testing"

	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/stretchr/testify/require"

	"github.com/bluenviron/mediamtx/internal/conf"
	"github.com/bluenviron/mediamtx/internal/defs"
	"github.com/bluenviron/mediamtx/internal/test"
)

func TestPathManagerConfigGeneration(t *testing.T) {
	for _, confName := range []string{"mypath", "~^mypath$", "all_others"} {
		for _, change := range []string{"remove and restore", "replace and restore", "hot reload and restore",
			"unrelated path", "identical reload"} {
			t.Run(confName+"/"+change, func(t *testing.T) {
				pathConf := &conf.Path{Name: confName, Source: "publisher"}
				switch confName {
				case "~^mypath$":
					pathConf.Regexp = regexp.MustCompile("^mypath$")
				case "all_others":
					pathConf.Regexp = regexp.MustCompile("^.*$")
				}
				clonePathConf := func() *conf.Path {
					cloned := pathConf.Clone()
					// Validate normally rebuilds the compiled regexp after cloning.
					if pathConf.Regexp != nil {
						cloned.Regexp = regexp.MustCompile(pathConf.Regexp.String())
					}
					return cloned
				}
				otherConf := &conf.Path{Name: "other", Source: "publisher"}
				pm := &pathManager{
					authManager: test.NilAuthManager,
					pathConfs: map[string]*conf.Path{
						confName: pathConf,
						"other":  otherConf,
					},
					parent: test.NilLogger,
				}
				pm.initialize()
				defer pm.close()

				find := func(name string) *defs.PathFindPathConfRes {
					t.Helper()
					res, err := pm.FindPathConf(defs.PathFindPathConfReq{
						AccessRequest: defs.PathAccessRequest{Name: name, Publish: true},
					})
					require.NoError(t, err)
					require.NotZero(t, res.ConfigGeneration)
					return res
				}
				captured := find("mypath")
				otherCaptured := find("other")
				require.NotEqual(t, captured.ConfigGeneration, otherCaptured.ConfigGeneration)

				switch change {
				case "remove and restore":
					pm.ReloadPathConfs(map[string]*conf.Path{"other": otherConf.Clone()})
					// This request runs after the reload on the manager goroutine,
					// confirming absence before the byte-identical config returns.
					_, err := pm.FindPathConf(defs.PathFindPathConfReq{
						AccessRequest: defs.PathAccessRequest{Name: "mypath", Publish: true},
					})
					require.EqualError(t, err, "path 'mypath' is not configured")
					pm.ReloadPathConfs(map[string]*conf.Path{
						confName: clonePathConf(),
						"other":  otherConf.Clone(),
					})

				case "replace and restore", "hot reload and restore":
					replacement := clonePathConf()
					if change == "hot reload and restore" {
						replacement.RecordPath = "different"
						require.True(t, pathConfCanBeUpdated(pathConf, replacement))
					} else {
						replacement.OverridePublisher = true
						require.False(t, pathConfCanBeUpdated(pathConf, replacement))
					}
					pm.ReloadPathConfs(map[string]*conf.Path{confName: replacement, "other": otherConf.Clone()})
					require.Greater(t, find("mypath").ConfigGeneration, captured.ConfigGeneration)
					pm.ReloadPathConfs(map[string]*conf.Path{
						confName: clonePathConf(),
						"other":  otherConf.Clone(),
					})

				case "unrelated path":
					replacement := otherConf.Clone()
					replacement.OverridePublisher = true
					pm.ReloadPathConfs(map[string]*conf.Path{confName: clonePathConf(), "other": replacement})

				case "identical reload":
					pm.ReloadPathConfs(map[string]*conf.Path{
						confName: clonePathConf(),
						"other":  otherConf.Clone(),
					})
				}

				current := find("mypath")
				require.True(t, current.Conf.Equal(captured.Conf))
				changed := change != "unrelated path" && change != "identical reload"
				if changed {
					require.Greater(t, current.ConfigGeneration, captured.ConfigGeneration)
					require.Greater(t, current.ConfigGeneration, otherCaptured.ConfigGeneration)
				} else {
					require.Equal(t, captured.ConfigGeneration, current.ConfigGeneration)
				}

				add := func(name string, expectation *defs.PathFindPathConfRes) error {
					t.Helper()
					_, err := pm.AddPublisher(defs.PathAddPublisherReq{
						Author:                   &dummyPublisher{},
						Desc:                     &description.Session{},
						ConfToCompare:            expectation.Conf,
						ExpectedConfigGeneration: expectation.ConfigGeneration,
						AccessRequest: defs.PathAccessRequest{
							Name: name, Publish: true, SkipAuth: true,
						},
					})
					return err
				}
				if changed {
					require.EqualError(t, add("mypath", captured), "configuration has changed")
					require.NoError(t, add("mypath", current))
				} else {
					require.NoError(t, add("mypath", captured))
				}

				otherCurrent := find("other")
				if change == "unrelated path" {
					require.Greater(t, otherCurrent.ConfigGeneration, otherCaptured.ConfigGeneration)
					require.EqualError(t, add("other", otherCaptured), "configuration has changed")
				} else {
					require.Equal(t, otherCaptured.ConfigGeneration, otherCurrent.ConfigGeneration)
					require.NoError(t, add("other", otherCaptured))
				}
			})
		}
	}
}

func TestPathManagerConfigGenerationAndConfComparison(t *testing.T) {
	pm := &pathManager{
		authManager: test.NilAuthManager,
		pathConfs: map[string]*conf.Path{
			"mypath": {Name: "mypath", Source: "publisher"},
		},
		parent: test.NilLogger,
	}
	pm.initialize()
	defer pm.close()

	captured, err := pm.FindPathConf(defs.PathFindPathConfReq{
		AccessRequest: defs.PathAccessRequest{Name: "mypath", Publish: true},
	})
	require.NoError(t, err)

	changedConf := captured.Conf.Clone()
	changedConf.OverridePublisher = true
	_, err = pm.AddPublisher(defs.PathAddPublisherReq{
		ConfToCompare:            changedConf,
		ExpectedConfigGeneration: captured.ConfigGeneration,
		AccessRequest:            defs.PathAccessRequest{Name: "mypath", Publish: true, SkipAuth: true},
	})
	require.EqualError(t, err, "configuration has changed")

	// A direct publisher has no earlier authorization to compare against and
	// is authenticated by AddPublisher, as with MoQ.
	_, err = pm.AddPublisher(defs.PathAddPublisherReq{
		Author:        &dummyPublisher{},
		Desc:          &description.Session{},
		AccessRequest: defs.PathAccessRequest{Name: "mypath", Publish: true},
	})
	require.NoError(t, err)
}

func TestPathManagerConfigGenerationNoAlloc(t *testing.T) {
	pm := &pathManager{
		authManager: test.NilAuthManager,
		pathConfs: map[string]*conf.Path{
			"mypath": {Name: "mypath", Source: "publisher"},
		},
		parent: test.NilLogger,
	}
	pm.initialize()
	defer pm.close()

	captured, err := pm.FindPathConf(defs.PathFindPathConfReq{
		AccessRequest: defs.PathAccessRequest{Name: "mypath", Publish: true},
	})
	require.NoError(t, err)

	// Reuse the request channel to measure admission on the manager goroutine,
	// excluding AddPublisher's existing channel and stream allocations.
	req := defs.PathAddPublisherReq{
		ConfToCompare:            captured.Conf,
		ExpectedConfigGeneration: captured.ConfigGeneration,
		AccessRequest:            defs.PathAccessRequest{Name: "mypath", Publish: true, SkipAuth: true},
		Res:                      make(chan defs.PathAddPublisherRes),
	}
	var res defs.PathAddPublisherRes
	allocs := testing.AllocsPerRun(100, func() {
		pm.chAddPublisher <- req
		res = <-req.Res
		if res.Path != nil {
			res.Path.(*path).pendingRequests.Add(-1)
		}
	})
	require.NoError(t, res.Err)
	require.NotNil(t, res.Path)
	require.Zero(t, allocs)
}
