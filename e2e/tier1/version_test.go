package tier1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"cmm/e2e/harness"
	"cmm/e2e/mockserver"
	"cmm/internal/selfupdate"
)

func TestVersion_UpToDate(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := selfupdate.ReleaseInfo{
			TagName: "v0.1.0",
		}
		json.NewEncoder(w).Encode(rel)
	}))
	defer ts.Close()

	ctx.SetEnv("CMM_UPDATE_URL", ts.URL)

	res := ctx.Run("version")
	res.AssertSuccess()
	res.AssertStdoutContains("Cloud Mod Manager is up to date")
}

func TestVersion_UpdateAvailable_CheckOnly(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := selfupdate.ReleaseInfo{
			TagName: "v0.2.0",
			HTMLURL: "https://github.com/aegeada/cloudModManager/releases/tag/v0.2.0",
		}
		json.NewEncoder(w).Encode(rel)
	}))
	defer ts.Close()

	ctx.SetEnv("CMM_UPDATE_URL", ts.URL)

	res := ctx.Run("version", "--check")
	res.AssertSuccess()
	res.AssertStdoutContains("A new version of Cloud Mod Manager is available: v0.2.0")
}

func TestVersion_UpdateCancelled(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := selfupdate.ReleaseInfo{
			TagName: "v0.2.0",
		}
		json.NewEncoder(w).Encode(rel)
	}))
	defer ts.Close()

	ctx.SetEnv("CMM_UPDATE_URL", ts.URL)

	res := ctx.RunWithStdin("n\n", "version")
	res.AssertSuccess()
	res.AssertStdoutContains("Update cancelled.")
}
