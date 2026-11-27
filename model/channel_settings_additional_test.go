/*
Copyright (C) 2023-2026 QuantumNous
Copyright (C) 2026 CubeRouter

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

package model

import (
	"os"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedInvalidSettingsChannel(t *testing.T) (*Channel, string) {
	t.Helper()
	invalid := `{"broken":`
	setting := invalid
	channel := &Channel{
		Id:            9001,
		Name:          "invalid-settings",
		Key:           "sk-test",
		Type:          1,
		Status:        common.ChannelStatusEnabled,
		Setting:       &setting,
		OtherSettings: invalid,
	}
	require.NoError(t, DB.Create(channel).Error)
	return channel, invalid
}

// Reading a cached channel's settings must not rewrite the shared object when
// its stored JSON is invalid. Concurrent relay and polling readers share the
// same *Channel; mutating Setting/OtherSettings here used to race and could
// even nil-deref another reader.
func TestChannelSettingsAccessorsConfiguredDatabases(t *testing.T) {
	dialects := []string{"sqlite"}
	if os.Getenv("TEST_MYSQL_DSN") != "" {
		dialects = append(dialects, "mysql")
	}
	if os.Getenv("TEST_POSTGRES_DSN") != "" {
		dialects = append(dialects, "postgres")
	}

	for _, dialect := range dialects {
		t.Run(dialect, func(t *testing.T) {
			db := openConfiguredModelTestDB(t, dialect)
			originalDB := DB
			DB = db
			t.Cleanup(func() { DB = originalDB })
			require.NoError(t, db.AutoMigrate(&Channel{}))

			channel, invalid := seedInvalidSettingsChannel(t)
			assert.Equal(t, dto.ChannelSettings{}, channel.GetSetting())
			require.NotNil(t, channel.Setting)
			assert.Equal(t, invalid, *channel.Setting)
			assert.Equal(t, dto.ChannelOtherSettings{}, channel.GetOtherSettings())
			assert.Equal(t, invalid, channel.OtherSettings)

			var stored Channel
			require.NoError(t, db.First(&stored, channel.Id).Error)
			require.NotNil(t, stored.Setting)
			assert.Equal(t, invalid, *stored.Setting)
			assert.Equal(t, invalid, stored.OtherSettings)

			validSetting := `{"force_format":true}`
			validOther := `{"disable_task_polling_sleep":true}`
			channel.Setting = &validSetting
			channel.OtherSettings = validOther
			assert.True(t, channel.GetSetting().ForceFormat)
			assert.True(t, channel.GetOtherSettings().DisableTaskPollingSleep)
			require.NotNil(t, channel.Setting)
			assert.Equal(t, validSetting, *channel.Setting)
			assert.Equal(t, validOther, channel.OtherSettings)
		})
	}
}

func TestChannelSettingsAccessorsAreConcurrencySafe(t *testing.T) {
	truncateTables(t)
	channel, invalid := seedInvalidSettingsChannel(t)

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				channel.GetSetting()
				channel.GetOtherSettings()
			}
		}()
	}
	wg.Wait()

	require.NotNil(t, channel.Setting)
	assert.Equal(t, invalid, *channel.Setting)
	assert.Equal(t, invalid, channel.OtherSettings)
}
