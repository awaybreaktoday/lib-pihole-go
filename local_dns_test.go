package pihole

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testAssertDNS(t *testing.T, c *Client, expected *DNSRecord, assertErr error) {
	actual, err := c.LocalDNS.Get(context.TODO(), expected.Domain)
	if assertErr != nil {
		assert.ErrorAs(t, err, assertErr)
		return
	}

	require.NoError(t, err)

	assert.Equal(t, expected.Domain, actual.Domain)
	assert.Equal(t, expected.IP, actual.IP)
	assert.Equal(t, expected.Comment, actual.Comment)
	assert.Equal(t, expected.HasTTL, actual.HasTTL)
	if expected.HasTTL {
		assert.Equal(t, expected.TTL, actual.TTL)
	}
}

func cleanupDNS(t *testing.T, c *Client, domain string) {
	if err := c.LocalDNS.Delete(context.TODO(), domain); err != nil {
		log.Printf("Failed to clean up domain record: %s\n", domain)
	}
}

func TestLocalDNS(t *testing.T) {
	t.Run("Test create a DNS record", func(t *testing.T) {
		isAcceptance(t)

		c := newTestClient(t)
		defer cleanupTestClient(c)

		domain := fmt.Sprintf("test.%s", randomID())

		record, err := c.LocalDNS.Create(context.Background(), domain, "127.0.0.1")
		require.NoError(t, err)

		defer cleanupDNS(t, c, domain)

		testAssertDNS(t, c, record, nil)
		testAssertDNS(t, c, &DNSRecord{
			Domain: record.Domain,
			IP:     "127.0.0.1",
		}, nil)
	})

	t.Run("Test delete a DNS record", func(t *testing.T) {
		isAcceptance(t)

		c := newTestClient(t)
		defer cleanupTestClient(c)

		ctx := context.Background()

		domain := fmt.Sprintf("test.%s", randomID())

		record, err := c.LocalDNS.Create(ctx, domain, "127.0.0.1")
		require.NoError(t, err)
		defer cleanupDNS(t, c, record.Domain)

		err = c.LocalDNS.Delete(ctx, domain)
		require.NoError(t, err)

		_, err = c.LocalDNS.Get(ctx, domain)
		assert.ErrorIs(t, err, ErrorLocalDNSNotFound)
	})
}

func TestDNSRecordListResponse_toDNSRecordList(t *testing.T) {
	t.Run("parses records with extra spacing", func(t *testing.T) {
		resp := dnsRecordListResponse{
			Config: dnsRecordConfigListResponse{
				DNS: dnsRecordDNSListResponse{
					Hosts: []string{"127.0.0.1    example.com"},
				},
			},
		}

		records, err := resp.toDNSRecordList()
		require.NoError(t, err)
		require.Len(t, records, 1)
		assert.Equal(t, "127.0.0.1", records[0].IP)
		assert.Equal(t, "example.com", records[0].Domain)
		assert.False(t, records[0].HasTTL)
		assert.Empty(t, records[0].Comment)
	})

	t.Run("parses records with ttl and comment", func(t *testing.T) {
		resp := dnsRecordListResponse{
			Config: dnsRecordConfigListResponse{
				DNS: dnsRecordDNSListResponse{
					Hosts: []string{"127.0.0.1 example.com 3600 # test comment"},
				},
			},
		}

		records, err := resp.toDNSRecordList()
		require.NoError(t, err)
		require.Len(t, records, 1)
		assert.True(t, records[0].HasTTL)
		assert.Equal(t, 3600, records[0].TTL)
		assert.Equal(t, "test comment", records[0].Comment)
	})

	t.Run("returns an error for invalid records", func(t *testing.T) {
		resp := dnsRecordListResponse{
			Config: dnsRecordConfigListResponse{
				DNS: dnsRecordDNSListResponse{
					Hosts: []string{"127.0.0.1"},
				},
			},
		}

		_, err := resp.toDNSRecordList()
		require.Error(t, err)
	})
}

func TestLocalDNS_CreateReturnsAPIError(t *testing.T) {
	isUnit(t)

	httpClient := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPut && strings.HasPrefix(req.URL.Path, "/api/config/dns/hosts/"):
			return newHTTPResponse(http.StatusBadRequest, `{"error":{"key":"bad_request","message":"duplicate","hint":null}}`), nil
		default:
			return newHTTPResponse(http.StatusNotFound, ``), nil
		}
	})}

	client, err := New(Config{
		BaseURL:    "http://pi.test",
		SessionID:  "test",
		HttpClient: httpClient,
	})
	require.NoError(t, err)

	_, err = client.LocalDNS.Create(context.Background(), "example.com", "127.0.0.1")
	var apiErr *DNSAPIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	assert.Equal(t, "bad_request", apiErr.Key)
	assert.Equal(t, "duplicate", apiErr.Message)
}
