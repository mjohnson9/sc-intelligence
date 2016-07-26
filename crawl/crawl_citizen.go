package crawl

import (
	"fmt"
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

	/*for _, org := range citizen.Organizations {
		if len(org.SID) == 0 || org.SID != "SUN" {
			continue
		}

		err = maybeCrawlOrg(c, org.SID)
		if err != nil {
			panic(err)
		}
	}*/

	log.Debugf(c, "updating citizen")
	err = nds.RunInTransaction(c, func(tc context.Context) error {
		return updateCitizen(tc, citizen)
	}, &datastore.TransactionOptions{XG: true})
	if err != nil {
		panic(err)
	}
}

func updateCitizen(c context.Context, citizen *starcitizen.Citizen) error {
	datastoreCitizen, err := models.GetCitizen(c, citizen.UEENumber)
	if err != nil {
		return err
	}

	if datastoreCitizen == nil {
		return insertNewCitizen(c, citizen)
	}

	return fmt.Errorf("not yet implemented")
}

func insertNewCitizen(c context.Context, citizen *starcitizen.Citizen) error {
	curTime := time.Now()

	newCitizen := &models.Citizen{
		ID:            citizen.UEENumber,
		Handle:        citizen.Handle,
		Moniker:       citizen.Moniker,
		Organizations: makeOrgList(citizen),
	}

	historyItems := make([]*models.CitizenHistory, 0, len(citizen.Organizations)+3)

	historyItems = append(historyItems,
		&models.CitizenHistory{
			WhatChanged: models.ChangedCrawl,
			Timestamp:   curTime,
		},
		&models.CitizenHistory{
			WhatChanged: models.ChangedHandle,
			NewValue:    newCitizen.Handle,
			Timestamp:   curTime,
		},
		&models.CitizenHistory{
			WhatChanged: models.ChangedMoniker,
			NewValue:    newCitizen.Moniker,
			Timestamp:   curTime,
		})

	for _, org := range newCitizen.Organizations {
		historyItems = append(historyItems, &models.CitizenHistory{
			WhatChanged: models.ChangedOrgJoined,
			NewValue:    org,
			Timestamp:   curTime,
		})
	}

	/*err := clearCitizenMoniker(c, newCitizen.Handle)
	if err != nil {
		return err
	}*/

	err := models.PutCitizen(c, newCitizen, historyItems, true)
	if err != nil {
		return err
	}

	t := createCitizenRecrawlTask(newCitizen.Handle)
	_, err = taskqueue.Add(c, t, "crawl-citizen")
	if err != nil {
		return err
	}

	return nil
}

func makeOrgList(citizen *starcitizen.Citizen) []string {
	orgs := make([]string, 0, len(citizen.Organizations))

	for _, citizenOrg := range citizen.Organizations {
		if citizenOrg.Visibility != "visible" {
			continue
		}

		orgs = append(orgs, citizenOrg.SID)
	}

	sort.Strings(orgs)

	return orgs
}

func hasOrg(orgs []string, org string) bool {
	index := sort.SearchStrings(orgs, org)

	if index >= len(orgs) {
		return false
	}

	return (orgs[index] == org)
}

func compareOrgs(oldOrgs, currentOrgs []string) (added []string, removed []string) {
	added = make([]string, 0, len(currentOrgs))
	removed = make([]string, 0, len(oldOrgs))

	for _, org := range currentOrgs {
		if !hasOrg(oldOrgs, org) {
			added = append(added, org)
		}
	}

	for _, org := range oldOrgs {
		if !hasOrg(currentOrgs, org) {
			removed = append(removed, org)
		}
	}

	return
}

func clearCitizenMoniker(c context.Context, handle string) error {
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

	newHistoryItem := &models.CitizenHistory{
		WhatChanged: models.ChangedHandle,
		NewValue:    "",
		Timestamp:   time.Now(),
	}

	err = models.PutCitizen(c, citizen, []*models.CitizenHistory{newHistoryItem}, false)
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
