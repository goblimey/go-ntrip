package reportfeed

import (
	"regexp"
	"testing"

	circularQueue "github.com/goblimey/go-ntrip/apps/proxy/circular_queue"

	"github.com/goblimey/go-tools/logger"
)

// TestSanitise tests the Sanitise function.
func TestSanitise(t *testing.T) {
	const expectedResult = "&lt;div&gt;&lt;/div&gt;"
	str := "<div></div>"

	result := Sanitise(str)

	if result != expectedResult {
		t.Errorf("Expected sanitised result to be \"%s\", got \"%s\"",
			expectedResult, string(result))
	}
}

// TestStatus tests the Status function.
func TestStatus(t *testing.T) {
	const expectedResultRegex = `
<h3>Last Client Buffer</h3>
<span id='clienttimestamp'>From Client [0]:
[ :a-zA-Z0-9]*</span>
<pre>
<code>
<div class="preformatted" id='clientbuffer'>
00000000  66 6f  |fo|
</div>
</code>
</pre>
<h3>Last Server Buffer</h3>
<span id='servertimestamp'>To Server [1]:
[ :a-zA-Z0-9]*</span>
<pre>
<code>
<div class="preformatted" id='serverbuffer'>
00000000  3c 62 61 72 3e   |&lt;bar&gt;|
</div>
</code>
</pre>
`
	regex := regexp.MustCompile(reduceString(expectedResultRegex))
	clientBuffer := []byte("foo")
	serverBuffer := []byte("<bar>")

	log := logger.New()
	q := circularQueue.NewCircularQueue(1)
	reportFeed := New(log, q)

	// Record only two characters of the client buffer.
	reportFeed.RecordClientBuffer(&clientBuffer, 0, 2)
	// Record all of the server buffer
	reportFeed.RecordServerBuffer(&serverBuffer, 1, len(serverBuffer))

	result := reportFeed.Status()

	reducedResult := reduceString(string(result))
	if !regex.Match([]byte(reducedResult)) {
		t.Errorf("Expected status report to match \"%v\", got \"%s\"",
			regex, reducedResult)
	}
}

// reduceString removes all newlines and reduces all other white space to a single space.
func reduceString(str string) string {
	re := regexp.MustCompile(`(?)\n+`)
	str = re.ReplaceAllString(str, "")
	re = regexp.MustCompile(`[ \t]+`)
	return re.ReplaceAllString(str, " ")
}
