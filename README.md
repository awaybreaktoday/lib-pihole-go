# go-pihole

A Golang Pi-hole client

Requires Pi-hole Web Interface >= `6`. For <6, use tag <= v0.0.4

## Usage

```go
import (
	"context"
	"errors"
	"log"
	"os"

	"github.com/awaybreaktoday/go-pihole"
)

client, err := pihole.New(pihole.Config{
	BaseURL:  "https://pi.hole/api",
	APIToken: os.Getenv("PIHOLE_API_TOKEN"),
	// Password remains supported for session logins when no API token is provided.
	Password: os.Getenv("PIHOLE_PASSWORD"),
})
if err != nil {
	log.Fatal(err)
}

record, err := client.LocalDNS.Create(context.Background(), "my-domain.com", "127.0.0.1")
if err != nil {
	log.Fatal(err)
}
log.Printf("%s -> %s (ttl=%d comment=%q)", record.Domain, record.IP, record.TTL, record.Comment)

alias, err := client.LocalCNAME.CreateRecord(context.Background(), &pihole.CNAMERecord{
	Domain: "www.example.com",
	Target: "example.com",
	TTL:    3600,
	HasTTL: true,
})
if err != nil {
	log.Fatal(err)
}
log.Printf("%s -> %s (ttl=%d)", alias.Domain, alias.Target, alias.TTL)

_, err = client.LocalDNS.Create(context.Background(), record.Domain, record.IP)
if err != nil {
	var dnsErr *pihole.DNSAPIError
	if errors.As(err, &dnsErr) {
		log.Printf("pihole rejected request: key=%s message=%s", dnsErr.Key, dnsErr.Message)
	}
}
```

### Authentication

`Config` accepts either `APIToken` or `APIKey` to enable Pi-hole's API token authentication. When supplied, the client automatically sends the `X-FTL-APIKEY` header and skips session negotiation. Supplying `Password` continues to work for legacy session-based flows, providing a fallback when no token is present.

### DNS and CNAME helpers

- `DNSRecord` now exposes optional `TTL` and `Comment` fields so callers can observe and persist Pi-hole's additional metadata.
- `CNAMERecord` tracks whether a TTL is supplied (`HasTTL`) and retains Pi-hole's original tuple, ensuring deletes round-trip exactly what the server expects.
- Use `LocalCNAME.CreateRecord` to submit a structured `CNAMERecord` and include TTLs when required.

Mutation helpers in both packages return typed errors (`*DNSAPIError`, `*CNAMEAPIError`) that surface Pi-hole's structured `error.key`, `message`, and `hint` values for improved diagnostics.

## Test

```sh
make test
```

### Acceptance

```sh
docker compose up -d
export PIHOLE_URL=http://localhost:8080
export PIHOLE_PASSWORD=test
make acceptance
```
