# DuckDuckGo Bot Challenge Handling

When the DuckDuckGo HTML endpoint decides a query looks automated it returns an
"anomaly" page rather than standard search results. The page includes
`#challenge-form`, `.anomaly-modal__modal`, and the copy
"Unfortunately, bots use DuckDuckGo too.". The CLI used to treat this response
as if no search results were found, which hid the real reason the request
failed.

Starting October 28, 2025 the Duck tool explicitly detects this challenge
markup. Instead of returning an empty list it now surfaces a soft error with the
message:

```
DuckDuckGo returned a bot challenge; results are unavailable until the
challenge is completed manually.
```

Callers can test for this condition by using `duckduck.IsChallengeError(err)`
and decide whether to retry later or fall back to another search provider.
