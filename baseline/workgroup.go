/*
 * Copyright 2017-2022 Provide Technologies Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package baseline

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/jinzhu/gorm"
	dbconf "github.com/kthomas/go-db-config"
	"github.com/kthomas/go-redisutil"
	uuid "github.com/kthomas/go.uuid"
	"github.com/provideplatform/baseline/common"
	provide "github.com/provideplatform/provide-go/api"
	"github.com/provideplatform/provide-go/api/baseline"
)

const requireCounterpartiesSleepInterval = time.Second * 15
const requireCounterpartiesTickerInterval = time.Second * 30 // HACK

// Workgroup is a baseline workgroup prototype
type Workgroup struct {
	baseline.Workgroup
	Name           *string        `json:"name"`
	Description    *string        `json:"description"`
	OrganizationID *uuid.UUID     `json:"-"`
	Participants   []*Participant `sql:"-" json:"participants,omitempty"`
	Workflows      []*Workflow    `sql:"-" json:"workflows,omitempty"`
}

// FindWorkgroupByID retrieves a workgroup for the given id
func FindWorkgroupByID(id uuid.UUID) *Workgroup {
	db := dbconf.DatabaseConnection()
	workgroup := &Workgroup{}
	db.Where("id = ?", id.String()).Find(&workgroup)
	if workgroup == nil || workgroup.ID == uuid.Nil {
		return nil
	}
	return workgroup
}

func init() {
	redisutil.RequireRedis()
}

func LookupBaselineWorkgroup(identifier string) *Workgroup {
	var workgroup *Workgroup

	key := fmt.Sprintf("baseline.workgroup.%s", identifier)
	raw, err := redisutil.Get(key)
	if err != nil {
		common.Log.Debugf("no baseline workgroup cached for key: %s; %s", key, err.Error())
		return nil
	}

	json.Unmarshal([]byte(*raw), &workgroup)
	return workgroup
}

func (w *Workgroup) Create() bool {
	if !w.Validate() {
		return false
	}

	newRecord := w.ID == uuid.Nil || FindWorkgroupByID(w.ID) == nil
	success := false

	if newRecord {
		// make it a tx?
		db := dbconf.DatabaseConnection()
		result := db.Create(&w)
		rowsAffected := result.RowsAffected
		errors := result.GetErrors()
		if len(errors) > 0 {
			for _, err := range errors {
				w.Errors = append(w.Errors, &provide.Error{
					Message: common.StringOrNil(err.Error()),
				})
			}
		}

		// if FindWorkgroupByID(w.ID) == nil { // WHY??
		success = rowsAffected > 0
		// }
	}

	return success
}

func (w *Workgroup) participantsCount(tx *gorm.DB) int {
	rows, err := tx.Raw("SELECT count(*) FROM workgroups_participants WHERE workgroup_id=?", w.ID).Rows()
	if err != nil {
		common.Log.Warningf("failed to read workgroup participants count; %s", err.Error())
		return 0
	}

	var len int
	for rows.Next() {
		err = rows.Scan(&len)
		if err != nil {
			common.Log.Warningf("failed to read workgroup participants count; %s", err.Error())
			return 0
		}
	}

	return len
}

func (w *Workgroup) listParticipants(tx *gorm.DB) []*WorkgroupParticipant {
	participants := make([]*WorkgroupParticipant, 0)
	rows, err := tx.Raw("SELECT * FROM workgroups_participants WHERE workgroup_id=?", w.ID).Rows()
	if err != nil {
		common.Log.Warningf("failed to list workgroup participants; %s", err.Error())
		return participants
	}

	for rows.Next() {
		p := &WorkgroupParticipant{}
		err = tx.ScanRows(rows, &p)
		if err != nil {
			common.Log.Warningf("failed to list workgroup participants; %s", err.Error())
			return participants
		}
		participants = append(participants, p)
	}

	return participants
}

func (w *Workgroup) addParticipant(participant string, tx *gorm.DB) bool {
	common.Log.Debugf("adding participant %s to workgroup: %s", participant, w.ID)
	result := tx.Exec("INSERT INTO workgroups_participants (workgroup_id, participant) VALUES (?, ?)", w.ID, participant)
	success := result.RowsAffected == 1
	if success {
		common.Log.Debugf("added participant %s from workgroup: %s", participant, w.ID)
	} else {
		common.Log.Warningf("failed to add participant %s from workgroup: %s", participant, w.ID)
		errors := result.GetErrors()
		if len(errors) > 0 {
			for _, err := range errors {
				w.Errors = append(w.Errors, &provide.Error{
					Message: common.StringOrNil(err.Error()),
				})
			}
		}
	}

	return len(w.Errors) == 0
}

func (w *Workgroup) removeParticipant(participant string, tx *gorm.DB) bool {
	common.Log.Debugf("removing participant %s to workgroup: %s", participant, w.ID)
	result := tx.Exec("DELETE FROM workgroups_participants WHERE workgroup_id=? AND participant=?", w.ID, participant)
	success := result.RowsAffected == 1
	if success {
		common.Log.Debugf("removed participant %s from workgroup: %s", participant, w.ID)
	} else {
		common.Log.Warningf("failed to remove participant %s from workgroup: %s", participant, w.ID)
		errors := result.GetErrors()
		if len(errors) > 0 {
			for _, err := range errors {
				w.Errors = append(w.Errors, &provide.Error{
					Message: common.StringOrNil(err.Error()),
				})
			}
		}
	}

	return len(w.Errors) == 0
}

func (w *Workgroup) Validate() bool {
	if w.Name == nil {
		w.Errors = append(w.Errors, &provide.Error{
			Message: common.StringOrNil("name is required"),
		})
	}

	// if w.OrganizationID == nil {
	// 	w.Errors = append(w.Errors, &provide.Error{
	// 		Message: common.StringOrNil("organization_id is required"),
	// 	})
	// }

	return len(w.Errors) == 0
}
