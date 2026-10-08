package middleware

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
)

const TaskArtifactAccessContextKey = "task_artifact_access"

const (
	taskArtifactAccessRawContextKey       = "task_artifact_access_raw"
	taskArtifactAccessPresentContextKey   = "task_artifact_access_present"
	taskArtifactAccessInvalidContextKey   = "task_artifact_access_invalid"
	taskArtifactAccessRateWindow          = time.Minute
	taskArtifactAccessCleanupInterval     = time.Minute
	maxEncodedTaskArtifactAccessQuerySize = 128
)

type taskArtifactRateEntry struct {
	windowStart time.Time
	count       int
}

type taskArtifactAccessLimiter struct {
	mutex       sync.Mutex
	global      int
	byIP        map[string]int
	byObject    map[string]int
	rates       map[string]taskArtifactRateEntry
	nextCleanup time.Time
	limits      system_setting.TaskArtifactAccessLimits
}

var taskArtifactAnonymousLimiter = newTaskArtifactAccessLimiter(
	system_setting.LoadTaskArtifactAccessLimits(),
)

func newTaskArtifactAccessLimiter(limits system_setting.TaskArtifactAccessLimits) *taskArtifactAccessLimiter {
	return &taskArtifactAccessLimiter{
		byIP:     make(map[string]int),
		byObject: make(map[string]int),
		rates:    make(map[string]taskArtifactRateEntry),
		limits:   limits,
	}
}

func (l *taskArtifactAccessLimiter) invalidAttempt(now time.Time, ip string) bool {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if l.nextCleanup.IsZero() || !now.Before(l.nextCleanup) {
		for key, entry := range l.rates {
			if now.Sub(entry.windowStart) >= taskArtifactAccessRateWindow {
				delete(l.rates, key)
			}
		}
		l.nextCleanup = now.Add(taskArtifactAccessCleanupInterval)
	}

	rate := l.rates[ip]
	if rate.windowStart.IsZero() || now.Sub(rate.windowStart) >= taskArtifactAccessRateWindow {
		rate = taskArtifactRateEntry{windowStart: now}
	}
	if rate.count >= l.limits.InvalidRatePerMinute {
		return false
	}
	rate.count++
	l.rates[ip] = rate
	return true
}

func (l *taskArtifactAccessLimiter) acquire(ip, taskID, artifactKey string) (func(), bool) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	objectKey := taskID + "\x00" + artifactKey
	if l.global >= l.limits.GlobalConcurrency ||
		l.byIP[ip] >= l.limits.IPConcurrency ||
		l.byObject[objectKey] >= l.limits.ObjectConcurrency {
		return nil, false
	}

	l.global++
	l.byIP[ip]++
	l.byObject[objectKey]++

	var once sync.Once
	return func() {
		once.Do(func() {
			l.mutex.Lock()
			defer l.mutex.Unlock()
			l.global--
			l.byIP[ip]--
			l.byObject[objectKey]--
			if l.byIP[ip] == 0 {
				delete(l.byIP, ip)
			}
			if l.byObject[objectKey] == 0 {
				delete(l.byObject, objectKey)
			}
		})
	}, true
}

func redactTaskArtifactAccessQuery() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		isArtifactContent := strings.HasPrefix(path, "/v1/tasks/") &&
			strings.Contains(path, "/artifacts/") &&
			strings.HasSuffix(path, "/content")
		isLegacyVideoContent := strings.HasPrefix(path, "/v1/videos/") &&
			strings.HasSuffix(path, "/content")
		if !isArtifactContent && !isLegacyVideoContent {
			c.Next()
			return
		}

		rawAccess, present, invalid := popTaskArtifactAccessQuery(c.Request)
		if present {
			c.Set(taskArtifactAccessRawContextKey, rawAccess)
			c.Set(taskArtifactAccessPresentContextKey, true)
			c.Set(taskArtifactAccessInvalidContextKey, invalid)
		}
		c.Next()
	}
}

func popTaskArtifactAccessQuery(request *http.Request) (string, bool, bool) {
	if request == nil || request.URL == nil {
		return "", false, false
	}
	rawAccess := ""
	count := 0
	invalid := false
	kept := make([]string, 0)
	for _, part := range strings.Split(request.URL.RawQuery, "&") {
		rawKey, rawValue, _ := strings.Cut(part, "=")
		key, err := url.QueryUnescape(rawKey)
		if err != nil || key != service.TaskArtifactAccessQueryParameter {
			kept = append(kept, part)
			continue
		}
		count++
		if len(rawValue) > maxEncodedTaskArtifactAccessQuerySize {
			invalid = true
			continue
		}
		if count == 1 {
			value, decodeErr := url.QueryUnescape(rawValue)
			if decodeErr != nil {
				invalid = true
			} else {
				rawAccess = value
			}
		}
	}
	if count == 0 {
		return "", false, false
	}
	invalid = invalid || count != 1
	request.URL.RawQuery = strings.Join(kept, "&")
	request.RequestURI = request.URL.RequestURI()
	return rawAccess, true, invalid
}

// taskArtifactAccessOutcome is the result of evaluating a content request's
// route-bound capability.
type taskArtifactAccessOutcome int

const (
	// taskArtifactAccessAbsent means the request carried no access query at
	// all; the caller picks the fallback identity.
	taskArtifactAccessAbsent taskArtifactAccessOutcome = iota
	// taskArtifactAccessGranted means the capability verified and the request
	// was admitted under the anonymous capability limiter. The caller must
	// invoke the returned release once the request is done.
	taskArtifactAccessGranted
	// taskArtifactAccessDenied means a capability was present but rejected.
	// The rejection response has already been written.
	taskArtifactAccessDenied
)

// evaluateTaskArtifactAccess verifies a route-bound capability before any
// database read, then applies the anonymous concurrency limiter. It writes the
// rejection response itself when it returns taskArtifactAccessDenied, and
// returns a non-nil release for taskArtifactAccessGranted.
func evaluateTaskArtifactAccess(c *gin.Context, taskID, artifactKey string) (taskArtifactAccessOutcome, func()) {
	rawAccess := c.GetString(taskArtifactAccessRawContextKey)
	present := c.GetBool(taskArtifactAccessPresentContextKey)
	invalid := c.GetBool(taskArtifactAccessInvalidContextKey)
	if queryAccess, queryPresent, queryInvalid := popTaskArtifactAccessQuery(c.Request); queryPresent {
		present = true
		if rawAccess == "" {
			rawAccess = queryAccess
		}
		invalid = invalid || queryInvalid
	}
	if !present {
		return taskArtifactAccessAbsent, nil
	}

	c.Header("Cache-Control", "private, no-store")

	ip := c.ClientIP()
	if ip == "" {
		ip = "unknown"
	}
	if invalid || !service.VerifyTaskArtifactAccess(rawAccess, taskID, artifactKey) {
		if !taskArtifactAnonymousLimiter.invalidAttempt(time.Now(), ip) {
			writeTaskArtifactAccessLimited(c)
			return taskArtifactAccessDenied, nil
		}
		WriteTaskArtifactAccessNotFound(c)
		return taskArtifactAccessDenied, nil
	}

	release, ok := taskArtifactAnonymousLimiter.acquire(ip, taskID, artifactKey)
	if !ok {
		writeTaskArtifactAccessLimited(c)
		return taskArtifactAccessDenied, nil
	}

	c.Set(TaskArtifactAccessContextKey, true)
	return taskArtifactAccessGranted, release
}

// TokenOrTaskArtifactAccessAuth accepts the normal relay API Bearer token or a
// route-bound capability. Capabilities are verified before any database read.
func TokenOrTaskArtifactAccessAuth(taskParam, artifactParam string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "private, no-store")

		outcome, release := evaluateTaskArtifactAccess(c, c.Param(taskParam), c.Param(artifactParam))
		switch outcome {
		case taskArtifactAccessAbsent:
			TokenAuth()(c)
		case taskArtifactAccessGranted:
			defer release()
			c.Next()
		}
	}
}

// TokenOrVideoContentAccessAuth accepts a route-bound video capability, a
// dashboard session, or a relay API token. /v1/videos/{task_id}/content has no
// artifact path segment, so the capability is bound to the fixed
// service.TaskVideoArtifactKey.
//
// A valid capability is verified before any database read, which is what lets
// credential-less players (the dashboard's <video> tag and its blob download)
// fetch content without a session. Requests without an access query keep the
// pre-existing dashboard/token behaviour.
func TokenOrVideoContentAccessAuth(taskParam string) gin.HandlerFunc {
	return func(c *gin.Context) {
		outcome, release := evaluateTaskArtifactAccess(c, c.Param(taskParam), service.TaskVideoArtifactKey)
		switch outcome {
		case taskArtifactAccessAbsent:
			TokenOrUserAuth()(c)
		case taskArtifactAccessGranted:
			defer release()
			c.Next()
		}
	}
}

func IsTaskArtifactAccess(c *gin.Context) bool {
	return c != nil && c.GetBool(TaskArtifactAccessContextKey)
}

// WriteTaskArtifactAccessNotFound writes the neutral not-found response shared
// by every rejected or unresolved capability request. Controllers that serve a
// capability-authenticated lookup must reuse it so that a malformed task id and
// a genuinely missing task stay indistinguishable.
func WriteTaskArtifactAccessNotFound(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
		"error": gin.H{
			"message": "Task or artifact not found",
			"type":    "artifact_not_found",
			"code":    "artifact_not_found",
		},
	})
}

func writeTaskArtifactAccessLimited(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	c.Header("Retry-After", "60")
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
		"error": gin.H{
			"message": "Artifact access limit exceeded",
			"type":    "rate_limit_error",
			"code":    "artifact_access_limited",
		},
	})
}
