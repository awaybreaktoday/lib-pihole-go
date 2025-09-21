package pihole

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type LocalDNS interface {
	// List all DNS records.
	List(ctx context.Context) (DNSRecordList, error)

	// Create a DNS record.
	Create(ctx context.Context, domain string, IP string) (*DNSRecord, error)

	// Get a DNS record by its domain.
	Get(ctx context.Context, domain string) (*DNSRecord, error)

	// Delete a DNS record by its domain.
	Delete(ctx context.Context, domain string) error
}

var (
	ErrorLocalDNSNotFound = errors.New("local dns record not found")
)

type localDNS struct {
	client *Client
}

type DNSRecord struct {
	IP      string
	Domain  string
	TTL     int
	HasTTL  bool
	Comment string
	raw     string
}

type DNSRecordList []DNSRecord

type dnsRecordListResponse struct {
	Config dnsRecordConfigListResponse `json:"config"`
}

type dnsRecordConfigListResponse struct {
	DNS dnsRecordDNSListResponse `json:"dns"`
}

type dnsRecordDNSListResponse struct {
	Hosts []string `json:"hosts"`
}

func (res dnsRecordListResponse) toDNSRecordList() (DNSRecordList, error) {
	list := make(DNSRecordList, 0, len(res.Config.DNS.Hosts))

	for _, entry := range res.Config.DNS.Hosts {
		record, err := parseDNSRecord(entry)
		if err != nil {
			return nil, err
		}

		list = append(list, record)
	}

	return list, nil
}

func parseDNSRecord(raw string) (DNSRecord, error) {
	record := DNSRecord{raw: raw}
	line := strings.TrimSpace(raw)

	if line == "" {
		return record, fmt.Errorf("invalid DNS record: %q", raw)
	}

	commentIdx := strings.Index(line, "#")
	if commentIdx >= 0 {
		record.Comment = strings.TrimSpace(line[commentIdx+1:])
		line = strings.TrimSpace(line[:commentIdx])
	}

	parts := strings.Fields(line)
	if len(parts) < 2 {
		return record, fmt.Errorf("invalid DNS record: %q", raw)
	}

	record.IP = parts[0]
	record.Domain = parts[1]

	if len(parts) >= 3 {
		if ttl, err := strconv.Atoi(parts[2]); err == nil {
			record.TTL = ttl
			record.HasTTL = true
			if len(parts) > 3 {
				extra := strings.Join(parts[3:], " ")
				record.Comment = strings.TrimSpace(strings.Join([]string{record.Comment, extra}, " "))
			}
		} else {
			extra := strings.Join(parts[2:], " ")
			record.Comment = strings.TrimSpace(strings.Join([]string{record.Comment, extra}, " "))
		}
	}

	return record, nil
}

// List returns a list of custom DNS records
func (dns localDNS) List(ctx context.Context) (DNSRecordList, error) {
	res, err := dns.client.Get(ctx, "/api/config/dns/hosts")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var resList *dnsRecordListResponse
	if err := json.NewDecoder(res.Body).Decode(&resList); err != nil {
		return nil, fmt.Errorf("failed to parse customDNS list body: %w", err)
	}

	records, err := resList.toDNSRecordList()
	if err != nil {
		return nil, fmt.Errorf("failed to parse customDNS list body: %w", err)
	}

	return records, nil
}

// Create creates a custom DNS record
func (dns localDNS) Create(ctx context.Context, domain string, IP string) (*DNSRecord, error) {
	value := fmt.Sprintf("%s%%20%s", IP, domain)

	res, err := dns.client.Put(ctx, fmt.Sprintf("/api/config/dns/hosts/%s", value), nil)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(res.Body)
		return nil, newDNSAPIError(res.StatusCode, b)
	}

	// if !dnsRes.Success {
	// 	return nil, fmt.Errorf("failed to create DNS record %s %s : %s : %w", domain, IP, dnsRes.Message, err)
	// }

	return dns.Get(ctx, domain)
}

// Get returns a custom DNS record by its domain name
func (dns localDNS) Get(ctx context.Context, domain string) (*DNSRecord, error) {
	records, err := dns.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch custom DNS records: %w", err)
	}

	for _, record := range records {
		if strings.EqualFold(record.Domain, domain) {
			return &record, nil
		}
	}

	return nil, fmt.Errorf("%w: %s", ErrorLocalDNSNotFound, domain)
}

// Delete removes a custom DNS record
func (dns localDNS) Delete(ctx context.Context, domain string) error {
	record, err := dns.Get(ctx, domain)
	if err != nil {
		if errors.Is(err, ErrorLocalDNSNotFound) {
			return nil
		}

		return fmt.Errorf("failed looking up custom DNS record %s for deletion: %w", domain, err)
	}

	value := fmt.Sprintf("%s%%20%s", record.IP, record.Domain)

	res, err := dns.client.Delete(ctx, fmt.Sprintf("/api/config/dns/hosts/%s", value))
	if err != nil {
		return err
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(res.Body)
		return newDNSAPIError(res.StatusCode, b)
	}

	return nil
}
