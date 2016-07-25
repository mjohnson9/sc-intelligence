package crawl

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/julienschmidt/httprouter"
	"github.com/nightexcessive/sc-intelligence/models"
	"github.com/nightexcessive/starcitizen"
	"github.com/qedus/nds"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/taskqueue"
	"google.golang.org/appengine/urlfetch"
)

func crawlOrg(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	c := appengine.NewContext(req)

	orgSID := req.FormValue("org")
	if len(orgSID) == 0 {
		w.WriteHeader(400)
		io.WriteString(w, "\"org\" is a required POST value for this task")
		return
	}
	log.Debugf(c, "crawling organization profile: %s", orgSID)

	/*if lowercaseID := strings.ToLower(orgSID); lowercaseID != "sun" && lowercaseID != "pactinit" {
		panic("will only crawl PACTINIT and SUN")
	}*/

	org, err := starcitizen.RetrieveOrganization(urlfetch.Client(c), orgSID)
	if err == starcitizen.ErrMissing {
		log.Infof(c, "organization does not exist: %q", org.SID)
		return
	} else if err != nil {
		panic(err)
	}

	err = nds.RunInTransaction(c, func(tc context.Context) error {
		return updateOrg(tc, org)
	}, nil)
	if err != nil {
		panic(err)
	}

	members, err := starcitizen.RetrieveOrganizationMembers(urlfetch.Client(c), orgSID)
	if err != nil {
		panic(err)
	}

	for _, member := range members {
		err = maybeCrawlCitizen(c, member.Handle)
		if err != nil {
			panic(err)
		}
	}

	task := createOrgRecrawlTask(orgSID)
	_, err = taskqueue.Add(c, task, "crawl-org")
	if err != nil {
		panic(err)
	}
}

func updateOrg(c context.Context, org *starcitizen.Organization) error {
	curTime := time.Now()

	storedOrg, err := models.GetOrganization(c, org.SID)
	if err != nil {
		return err
	}

	if storedOrg == nil {
		storedOrg = &models.Organization{
			ID: org.SID,

			FirstSeen:   curTime,
			LastUpdated: curTime,
		}
	} else {
		storedOrg.LastUpdated = curTime
	}

	return models.PutOrganization(c, storedOrg)
}

func createOrgRecrawlTask(spectrumID string) *taskqueue.Task {
	const orgRecrawlDelay = 9 * 24 * time.Hour

	v := url.Values{}
	v.Set("org", spectrumID)

	t := taskqueue.NewPOSTTask("/task/crawl/org", v)
	t.Delay = orgRecrawlDelay

	return t
}

func maybeCrawlOrg(c context.Context, spectrumID string) error {
	organization, err := models.GetOrganization(c, spectrumID)
	if err != nil {
		return err
	}

	if organization != nil {
		// won't crawl because they already have been added to our index
		log.Debugf(c, "didn't add org %q to the crawl queue: already exists", spectrumID)
		return nil
	}

	task := createOrgRecrawlTask(spectrumID)
	task.Delay = time.Duration(0)
	task.Name = "org-first-crawl-" + strings.ToLower(spectrumID)

	_, err = taskqueue.Add(c, task, "crawl-org")
	if err == taskqueue.ErrTaskAlreadyAdded {
		log.Debugf(c, "didn't add org %q to the crawl queue: already in the queue", spectrumID)
		return nil
	} else if err != nil {
		return err
	}

	log.Debugf(c, "added org %q to the crawl queue", spectrumID)
	return nil
}
