# Context: Popcorn

The domain language for Popcorn, a self-hosted cinema-showtimes calendar for a
handful of chosen theaters. Implementation details live in code and ADRs, not
here.

## Glossary

### Theater
A cinema Popcorn follows, named by its allocine.fr **internal id** (`C0159`) and
a display **name**. The only thing the user configures by hand; everything else
is discovered from a theater. There is no interface for adding one -- the list
lives in the config file and a change means a restart, which is the right shape
for something that changes once a year.

A theater's name is the label the user reads and the key movies are grouped
under in a card, so two theaters must not share one.

### Window
The rolling range of days Popcorn shows, counted forward from today: seven by
default, one to thirty-one allowed. Days are addressed by their **offset** --
0 is today, 1 tomorrow -- never by date, so the calendar strip has no notion of
"the past". Yesterday's screenings are not history to browse; they are simply
outside the window.

The window is recomputed from the current time on every refresh rather than
fixed at startup, so a process left running for a week still calls the right day
"today".

### Showtime
One screening: a movie, at a theater, starting at a wall-clock time. The atom
everything else is built from. Times arrive without a zone and are Europe/Paris
local, which is why the container carries `tzdata` and a `TZ`.

A showtime may carry a **ticketing URL**, the deep link into the cinema's own
booking flow. Most do; a screening without one is still a screening, and the
interface just omits the link.

### Movie card
One movie as the user sees it for a given day: title, director, cast, genres,
runtime, synopsis, poster, and its screenings grouped by theater. Built by
**aggregation** -- the day's showtimes collapsed by movie, then by theater, with
each theater's times sorted. A movie playing at three theaters is one card with
three groups, not three cards.

Cards are identified by title. Two distinct films sharing a title would merge,
which has not happened and is the trade for not carrying allocine's ids through
the whole view layer.

### Want-to-see
Allocine's count of users who marked a film as wanted. Popcorn uses it only as a
sort key: cards are ordered by it, descending, so the day opens on what most
people are waiting for rather than on an alphabet. It is never shown.

### Original title
The international title, surfaced beside the French one only when it genuinely
differs. Allocine repeats the French title in that field for domestic films, so
a case-insensitive match counts as identical and nothing is printed. Its presence
is also what marks a film as foreign, which is how the trailer link knows to ask
for the **VOSTFR** cut -- original audio, French subtitles.

### Genre
A French genre label attached to a movie. The **catalogue** is the sorted union
of every genre with a screening somewhere in the window, and it drives the filter
chips: the interface only ever offers a genre something is actually playing in.
Filtering happens in the browser over cards already delivered, so it costs no
round-trip.

### Snapshot
The whole window's cards, built in one pass and swapped in atomically. Readers
always see a complete window, never a half-rebuilt one.

A snapshot is replaced only by a **successful** refresh. When every fetch in a
cycle fails, the previous snapshot keeps serving -- **serve-stale** -- because
showtimes a few hours old are far more useful than an empty page, and an
allocine outage must not take Popcorn down with it. Until the first snapshot
lands the service is live but **not ready**: it is starting, not broken.

### Refresh
One pass over the window: every day, every theater, fetched, aggregated and
swapped in. Runs on an interval (three hours by default) and once immediately at
startup. A single theater failing on a single day is logged and skipped; only a
cycle where nothing at all succeeded is a failed refresh.

### New release
A movie present in the fresh snapshot and absent from the one before it --
"newly at the affiche". Identified by title, deduplicated across the days of the
window, and ordered most-wanted first.

The first snapshot after a start announces nothing. On a cold start every movie
looks new, and a notification listing the entire week is noise, not news.

### Digest
The single notification a set of new releases becomes. It names up to three
titles and collapses the rest into a count, because a notification is a nudge to
open the calendar, not the calendar itself.

### Subscription
One browser's registration for notifications: the push service **endpoint** plus
the keys needed to encrypt to it. Created when the user accepts the browser
prompt, persisted to a file so a restart does not silently stop notifying, and
**pruned** when the push service reports the endpoint gone. An endpoint carries a
per-device secret, so only its host is ever logged.

Push is off unless a **VAPID** key pair is configured. Without it Popcorn is a
complete application -- installable, offline-capable, just silent -- so the
absence of keys is a supported configuration and not a misconfiguration.
