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

// GetSiteUsageTaskRowsByBucket aggregates every actor's tasks by local submit
// day or hour, actor, billing account, key, and model.
func GetSiteUsageTaskRowsByBucket(filter SiteUsageFilter, startTs, endTs, bucketSeconds int64) ([]SelfUsageAsyncRow, error) {
	return getUsageTaskRowsByBucket(usageAsyncScope{Site: true}, filter, startTs, endTs, bucketSeconds, 0, usageUserActor)
}

// GetSiteUsageMidjourneyRowsByBucket aggregates every actor's MJ tasks by
// local submit day or hour, actor, billing account, key, and model.
func GetSiteUsageMidjourneyRowsByBucket(filter SiteUsageFilter, startTs, endTs, bucketSeconds int64) ([]SelfUsageAsyncRow, error) {
	return getUsageMidjourneyRowsByBucket(usageAsyncScope{Site: true}, filter, startTs, endTs, bucketSeconds, 0, usageUserActor)
}
