package crawl

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/nightexcessive/sc-intelligence/models"
	"github.com/nightexcessive/starcitizen"
	"github.com/qedus/nds"
	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/taskqueue"
	"google.golang.org/appengine/urlfetch"
)

func crawlCitizen(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	c := appengine.NewContext(req)

	citizenHandle := req.FormValue("citizen")
	if len(citizenHandle) == 0 {
		w.WriteHeader(400)
		io.WriteString(w, "\"citizen\" is a required POST value for this task")
		return
	}
	log.Debugf(c, "crawling citizen profile: %s", citizenHandle)

	citizen, err := starcitizen.RetrieveCitizen(urlfetch.Client(c), citizenHandle)
	if err == starcitizen.ErrMissing {
		log.Debugf(c, "citizen profile doesn't exist on RSI, updating database")
		err = nds.RunInTransaction(c, func(tc context.Context) error {
			return clearCitizenMoniker(c, citizenHandle)
		}, nil)
		if err != nil {
			panic(err)
		}

		return
	} else if err != nil {
		panic(err)
	}

	for _, org := range citizen.Organizations {
		err = maybeCrawlOrg(c, org.SID)
		if err != nil {
			panic(err)
		}
	}

	log.Debugf(c, "updating citizen")
	err = nds.RunInTransaction(c, func(tc context.Context) error {
		return updateCitizen(tc, citizen)
	}, &datastore.TransactionOptions{XG: true})
	if err != nil {
		panic(err)
	}
}

func updateCitizen(c context.Context, citizen *starcitizen.Citizen) error {
	curTime := time.Now()

	datastoreCitizen, err := models.GetCitizen(c, citizen.UEENumber)
	if err != nil {
		return err
	}

	if datastoreCitizen == nil {
		datastoreCitizen = &models.Citizen{
			ID:      citizen.UEENumber,
			Handle:  citizen.Handle,
			Moniker: citizen.Moniker,

			FirstSeen:   curTime,
			LastUpdated: curTime,
		}
	} else {
		datastoreCitizen.LastUpdated = curTime
	}

	datastoreHistory, err := models.GetCitizenHistory(c, datastoreCitizen.ID)
	if err != nil {
		return err
	}

	if datastoreHistory == nil {
		datastoreHistory = &models.CitizenHistory{
			ID: datastoreCitizen.ID,

			Handles: []models.HistoryItem{
				models.HistoryItem{
					Value:     citizen.Handle,
					FirstSeen: curTime,
					LastSeen:  curTime,
				},
			},

			Monikers: []models.HistoryItem{
				models.HistoryItem{
					Value:     citizen.Moniker,
					FirstSeen: curTime,
					LastSeen:  curTime,
				},
			},
		}
	}

	if datastoreCitizen.Handle != citizen.Handle {
		err = clearCitizenMoniker(c, citizen.Handle)
		if err != nil {
			return err
		}

		datastoreCitizen.Handle = citizen.Handle

		datastoreHistory.Handles = append(datastoreHistory.Handles,
			models.HistoryItem{
				Value:     citizen.Handle,
				FirstSeen: curTime,
				LastSeen:  curTime,
			})
	} else {
		datastoreHistory.Handles[len(datastoreHistory.Handles)-1].LastSeen = curTime
	}

	if datastoreCitizen.Moniker != citizen.Moniker {
		datastoreCitizen.Moniker = citizen.Moniker

		datastoreHistory.Monikers = append(datastoreHistory.Monikers,
			models.HistoryItem{
				Value:     citizen.Moniker,
				FirstSeen: curTime,
				LastSeen:  curTime,
			})
	} else {
		datastoreHistory.Monikers[len(datastoreHistory.Monikers)-1].LastSeen = curTime
	}

	datastoreCitizen.LastUpdated = curTime

	err = models.PutCitizen(c, datastoreCitizen)
	if err != nil {
		return err
	}

	err = models.PutCitizenHistory(c, datastoreHistory)
	if err != nil {
		return err
	}

	t := createCitizenRecrawlTask(datastoreCitizen.Handle)
	_, err = taskqueue.Add(c, t, "crawl")
	if err != nil {
		return err
	}

	return nil
}

func clearCitizenMoniker(c context.Context, handle string) error {
	curTime := time.Now()

	citizen, err := models.FindCitizenByHandle(c, handle)
	if err != nil {
		return err
	}

	if citizen == nil {
		// no citizen with that handle
		log.Debugf(c, "found no citizen with the given handle")
		return nil
	}

	if strings.ToLower(citizen.Handle) != strings.ToLower(handle) {
		// this citizen's handle was already changed, but the query index hasn't
		// been updated yet
		log.Debugf(c, "citizen already updated")
		return nil
	}

	citizenHistory, err := models.GetCitizenHistory(c, citizen.ID)
	if err != nil {
		return err
	}

	citizenHistory.Handles = append(citizenHistory.Handles,
		models.HistoryItem{
			Value:     "",
			FirstSeen: curTime,
			LastSeen:  curTime,
		})

	citizen.Handle = ""
	citizen.LastUpdated = curTime

	err = models.PutCitizen(c, citizen)
	if err != nil {
		return err
	}

	err = models.PutCitizenHistory(c, citizenHistory)
	if err != nil {
		return err
	}

	log.Debugf(c, "updated the citizen previously owning the handle")

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

func maybeCrawlCitizen(c context.Context, handle string) error {
	citizen, err := models.FindCitizenByHandle(c, handle)
	if err != nil {
		return err
	}

	if citizen != nil {
		// won't crawl because they already have been added to our index
		return nil
	}

	task := createCitizenRecrawlTask(handle)
	task.Delay = time.Duration(0)
	task.Name = "citizen-first-crawl-" + strings.ToLower(handle)

	_, err = taskqueue.Add(c, task, "crawl")
	if err == taskqueue.ErrTaskAlreadyAdded {
		return nil
	} else if err != nil {
		return err
	}

	return nil
}
