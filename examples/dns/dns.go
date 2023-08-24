package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/miekg/dns"
)

func queryDNS(dnsAddr string, domain string) (string, error) {
	var resolved string
	client := dns.Client{}

	msg := dns.Msg{}
	msg.SetQuestion(domain+".", dns.TypeA)

	response, _, err := client.Exchange(&msg, dnsAddr)
	if err != nil {
		return resolved, err
	}

	if response.Rcode != dns.RcodeSuccess {
		return resolved, fmt.Errorf("DNS query failed: %s", dns.RcodeToString[response.Rcode])
	}

	for _, answer := range response.Answer {
		if aRecord, ok := answer.(*dns.A); ok {
			resolved = aRecord.A.String()
			break
		}
	}

	return resolved, nil
}

func setupDNS(dnsSrvAddr, domainName, domainIP string) error {
	loginReq := fmt.Sprintf("http://%s/api/user/login?user=admin&pass=admin&includeInfo=true", dnsSrvAddr)
	resp, err := http.Get(loginReq)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to login DNS: %d", resp.StatusCode)
	}

	bodyBuf, _ := io.ReadAll(resp.Body)

	type loginInfo struct {
		Token string `json:"token"`
	}
	logInfo := &loginInfo{}
	err = json.Unmarshal(bodyBuf, logInfo)
	if err != nil {
		return err
	}

	createZoneReq := fmt.Sprintf("http://%s/api/zones/create?token=%s&zone=%s&type=Primary", dnsSrvAddr, logInfo.Token, domainName)
	_, _ = http.Get(createZoneReq)

	addRecordReq := fmt.Sprintf("http://%s/api/zones/records/add?token=%s&domain=%s&zone=%s&ipAddress=%s&type=A", dnsSrvAddr, logInfo.Token, domainName, domainName, domainIP)
	resp, _ = http.Get(addRecordReq)
	bodyBuf, _ = io.ReadAll(resp.Body)
	type recordStatus struct {
		Status string `json:"status"`
	}
	record := &recordStatus{}
	err = json.Unmarshal(bodyBuf, record)

	if record.Status != "ok" {
		return fmt.Errorf("Failed to add %s with %s IP record to DNS, status %s", domainName, domainIP, record.Status)
	}

	fmt.Printf("Add: %s -> %s\n", domainName, domainIP)

	return nil
}
