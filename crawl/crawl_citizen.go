package crawl

import (
	"io"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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

func crawlCitizen(g *gin.Context) {
	c := appengine.NewContext(g.Request)

	citizenHandle := g.PostForm("citizen")
	if len(citizenHandle) == 0 {
		g.AbortWithStatus(400)
		io.WriteString(g.Writer, "\"citizen\" is a required POST value for this task")
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
		if len(org.SID) == 0 {
			continue
		}

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

	orgs := make([]string, 0, len(citizen.Organizations))

	for _, citizenOrg := range citizen.Organizations {
		if citizenOrg.Visibility != "visible" {
			continue
		}

		orgs = append(orgs, citizenOrg.SID)
	}

	sort.Strings(orgs)

	// keep the old organizations for comparison (they're assigned below)
	var oldOrganizations []string

	if datastoreCitizen == nil {
		datastoreCitizen = &models.Citizen{
			ID:            citizen.UEENumber,
			Handle:        citizen.Handle,
			Moniker:       citizen.Moniker,
			Organizations: orgs,

			FirstSeen:   curTime,
			LastUpdated: curTime,
		}

		oldOrganizations = orgs
	} else {
		datastoreCitizen.LastUpdated = curTime

		oldOrganizations = datastoreCitizen.Organizations
		datastoreCitizen.Organizations = orgs
	}

	datastoreHistory, err := models.GetCitizenHistory(c, datastoreCitizen.ID)
	if err != nil {
		return err
	}

	if datastoreHistory == nil {
		orgs := make([]models.HistoryItem, len(datastoreCitizen.Organizations))

		for i, sid := range datastoreCitizen.Organizations {
			orgs[i] = models.HistoryItem{
				Value:     sid,
				FirstSeen: curTime,
				LastSeen:  curTime,
			}
		}

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

			Organizations: orgs,
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

	for _, currentOrg := range citizen.Organizations {
		searchIndex := sort.SearchStrings(oldOrganizations, currentOrg.SID)
		knownOf := (searchIndex < len(oldOrganizations) && oldOrganizations[searchIndex] == currentOrg.SID)

		if knownOf {
			// they were in this organization in the last crawl; just update the
			// LastSeen time on the latest history entry
			historyIndex := -1

			for i := len(datastoreHistory.Organizations) - 1; i >= 0; i-- {
				if datastoreHistory.Organizations[i].Value == currentOrg.SID {
					historyIndex = i
					break
				}
			}

			if historyIndex < 0 {
				panic("strange condition: org is known but not in history")
			}

			datastoreHistory.Organizations[historyIndex].LastSeen = curTime
			continue
		}

		// the citizen was not in this organization in the last crawl; create a
		// new history entry
		datastoreHistory.Organizations = append(datastoreHistory.Organizations, models.HistoryItem{
			Value:     currentOrg.SID,
			FirstSeen: curTime,
			LastSeen:  curTime,
		})
	}

	err = models.PutCitizen(c, datastoreCitizen)
	if err != nil {
		return err
	}

	err = models.PutCitizenHistory(c, datastoreHistory)
	if err != nil {
		return err
	}

	t := createCitizenRecrawlTask(datastoreCitizen.Handle)
	_, err = taskqueue.Add(c, t, "crawl-citizen")
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

	_, err = taskqueue.Add(c, task, "crawl-citizen")
	if err == taskqueue.ErrTaskAlreadyAdded {
		return nil
	} else if err != nil {
		return err
	}

	return nil
}
