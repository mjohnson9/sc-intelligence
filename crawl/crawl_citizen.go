package crawl

import (
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/nightexcessive/sc-intelligence/models"
	"github.com/nightexcessive/starcitizen"
	"github.com/qedus/nds"
	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/taskqueue"
	"google.golang.org/appengine/urlfetch"
)

func crawlCitizen(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	c := appengine.NewContext(req)

	citizenHandle := req.FormValue("citizen")
	if len(citizenHandle) == 0 {
		w.WriteHeader(500)
		io.WriteString(w, "\"citizen\" is a required POST value for this task")
		return
	}

	citizen, err := starcitizen.RetrieveCitizen(urlfetch.Client(c), citizenHandle)
	if err != nil {
		panic(err)
	}

	storedCitizen, err := models.GetCitizen(c, citizen.UEENumber)
	if err != nil {
		panic(err)
	}

	if storedCitizen == nil {
		err := nds.RunInTransaction(c, func(tc context.Context) error {
			return insertCitizen(tc, citizen)
		}, nil)
		if err != nil {
			panic(err)
		}
		io.WriteString(w, "New citizen successfully crawled")
		return
	} else {
		err := nds.RunInTransaction(c, func(tc context.Context) error {
			return updateCitizen(tc, citizen)
		}, nil)
		if err != nil {
			panic(err)
		}
		io.WriteString(w, "Citizen successfully re-crawled")
		return
	}
}

func insertCitizen(c context.Context, citizen *starcitizen.Citizen) error {
	newCitizen := &models.Citizen{
		ID:      citizen.UEENumber,
		Handle:  citizen.Handle,
		Moniker: citizen.Moniker,

		LastUpdated: time.Now(),
	}

	err := models.PutCitizen(c, newCitizen)
	if err != nil {
		return err
	}

	v := url.Values{}
	v.Set("citizen", citizen.Handle)

	t := createCitizenRecrawlTask(newCitizen.Handle)
	_, err = taskqueue.Add(c, t, "crawl")
	if err != nil {
		return err
	}

	return nil
}

func updateCitizen(c context.Context, citizen *starcitizen.Citizen) error {
	oldCitizen, err := models.GetCitizen(c, citizen.UEENumber)
	if err != nil {
		return err
	}

	oldCitizen.Handle = citizen.Handle
	oldCitizen.Moniker = citizen.Moniker

	oldCitizen.LastUpdated = time.Now()

	err = models.PutCitizen(c, oldCitizen)
	if err != nil {
		return err
	}

	t := createCitizenRecrawlTask(oldCitizen.Handle)
	_, err = taskqueue.Add(c, t, "crawl")
	if err != nil {
		return err
	}

	return nil
}

func createCitizenRecrawlTask(citizenHandle string) *taskqueue.Task {
	const citizenRecrawlDelay = 29 * 24 * time.Hour

	v := url.Values{}
	v.Set("citizen", citizenHandle)

	t := taskqueue.NewPOSTTask("/task/crawl/citizen", v)
	t.Delay = citizenRecrawlDelay

	return t
}
