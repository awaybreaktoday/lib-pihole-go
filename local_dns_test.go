package pihole

import (
	"context"
	"fmt"
	"log"
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
