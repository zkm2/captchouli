package danbooru

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/bakape/captchouli/v2/common"
	"github.com/bakape/captchouli/v2/db"
	"github.com/olekukonko/tablewriter"
)

func TestMain(t *testing.M) {
	db.OpenForTests()
	os.Exit(t.Run())
}

func TestFetch(t *testing.T) {
	testFetches(t, "sakura_kyouko")
}

func TestFetchTimeout(t *testing.T) {
	if httpClient.Timeout <= 0 {
		t.Fatal("upstream HTTP client has no timeout")
	}
	previous := httpClient
	httpClient = &http.Client{Timeout: 100 * time.Millisecond}
	defer func() { httpClient = previous }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()

	const tag = "captchouli_timeout_test"
	for index := 0; index < 3; index++ {
		err := db.InsertPendingImage(db.PendingImage{
			MD5:       md5.Sum([]byte(fmt.Sprintf("%s_%d", tag, index))),
			TargetTag: tag,
			URL:       server.URL,
		})
		if err != nil {
			t.Fatal(err)
		}
		defer db.PopRandomPendingImage(tag)
	}

	file, _, err := Fetch(common.FetchRequest{Tag: tag})
	if file != nil {
		file.Close()
		os.Remove(file.Name())
		t.Error("timed-out download returned a temporary file")
	}
	var timeout net.Error
	if !errors.As(err, &timeout) || !timeout.Timeout() {
		t.Fatalf("expected body-read timeout, got %v", err)
	}
}

func testFetches(t *testing.T, tag string) {
	t.Helper()

	var buf bytes.Buffer
	w := tablewriter.NewWriter(&buf)
	w.SetAlignment(tablewriter.ALIGN_LEFT)
	w.SetColWidth(80)
	w.SetRowLine(true)
	w.SetHeader([]string{"rating", "MD5", "tags"})

	f, img, err := Fetch(common.FetchRequest{
		Tag: tag,
	})
	if err != nil {
		t.Fatal(err)
	}
	if f != nil {
		err = os.Remove(f.Name())
		if err != nil {
			t.Fatal(err)
		}
		err = f.Close()
		if err != nil {
			t.Fatal(err)
		}
		w.Append([]string{img.Rating.String(),
			hex.EncodeToString(img.MD5[:]),
			fmt.Sprint(img.Tags)})
	}

	w.Render()
	t.Logf("\n%s\n", buf.String())
}

func TestNoMatch(t *testing.T) {
	_, _, err := Fetch(common.FetchRequest{
		Tag: "sakura_kyouko_dsadsdadsadsad",
	})
	if err != common.ErrNoMatch {
		t.Fatal(err)
	}
}

func TestOnlyOnePage(t *testing.T) {
	testFetches(t, "symphogear_live")
}
