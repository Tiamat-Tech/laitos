package smtpd

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/dnsd"
)

const (
	// DNSBlackListQueryTimeoutSec is the number of seconds to be used as timeout when querying a DNS-based blacklist lookup server.
	DNSBlackListQueryTimeoutSec = 3
)

var (
	// SpamBlacklistLookupServers is a list of domain names that provide email spam reporting and blacklist look-up services.
	// Each of the domain name offers a DNS-based blacklist look-up service. By appending the reversed IPv4 address to any of
	// the domain names (e.g. resolve 4.3.2.1.domain.net to check blacklist status of 1.2.3.4), the success of DNS resolution
	// will indictate that the IP address has been blacklisted for spamming.
	SpamBlacklistLookupServers = []string{
		// www.barracudacentral.org "Barracuda Central maintains a history of IP addresses for both known spammers as well as senders with good email practices"
		"b.barracudacentral.org",
		// www.spamcop.net "SpamCop determines the origin of unwanted email and reports it to the relevant Internet service providers"
		"bl.spamcop.net",
		// www.abuseat.org "a division of spamhaus"
		"cbl.abuseat.org",
		// www.uceprotect.net "The project’s mission is to stop mail abuse, globally"
		"dnsbl-1.uceprotect.net",
		"dnsbl-2.uceprotect.net",
		"dnsbl-3.uceprotect.net",
		// www.sorbs.net "The SORBS (Spam and Open Relay Blocking System) provides free access to its DNS-based Block List"
		"dnsbl.sorbs.net",
		"spam.dnsbl.sorbs.net",
		// www.dnsbl.manitu.net "This publicly available DNS blacklist is permanently regenerated by the NiX Spam project"
		"ix.dnsbl.manitu.net",
		// www.gbudb.com "GBUdb is a real-time collaborative IP reputation system"
		"truncate.gbudb.net",
		// www.spamhaus.org "ZEN is the combination of all Spamhaus IP-based DNSBLs into one single powerful and comprehensive blocklist"
		"zen.spamhaus.org",
		// www.spamrats.com "We have several lists of IP Addresses that have all the indicators of being RATS, and you can use them just like any of your favourite RBLs."
		"noptr.spamrats.com",
		"spam.spamrats.com",
		// anonmails.de/dnsbl.php "To utilize the DNSBL in SpamAssasin, add the following ruleset to your local configuration file..."
		"spam.dnsbl.anonmails.de",
		// singular.ttk.pte.hu "SINGULARis is the PTE TTK Spam / Scam e-mail blacklist"
		"singular.ttk.pte.hu",
		// psbl.org "PSBL is an easy-on, easy-off blacklist that does not rely on testing and should reduce false positives"
		"psbl.surriel.com",
		// mailspike.org "Mailspike.org is a free IP Blocklist lookup service by MailSpike, the makers of Anubisnetworks Email Security Services"
		"z.mailspike.net",
		"bl.mailspike.net",
		// www.kempt.net/spam-policy.html "Kempt.net employs several methods to detect incoming spam and block the sending site"
		"dnsbl.kempt.net",
		// zapbl.net "ZapBL is a DNS-based realtime blacklist"
		"dnsbl.zapbl.net",
	}
)

// GetBlacklistLookupName returns a DNS name constructed from a combination of the suspect IP and blacklist
// lookup domain name. For example, in order to look-up a suspect IP 1.2.3.4 using blacklist look-up domain
// bl.spamcop.net, the function will return "4.3.2.1.bl.spamcop.net".
// The caller should then attempt to resolve the A record of the returned name. If the resolution is successful,
// then the suspect IP has been blacklisted by the look-up domain.
func GetBlacklistLookupName(suspectIP, blLookupDomain string) (string, error) {
	suspectIPv4 := net.ParseIP(suspectIP).To4()
	if suspectIPv4 == nil || len(suspectIPv4) < 4 {
		return "", fmt.Errorf("GetBlacklistLookupName: suspect IP %s does not appear to be a valid IPv4 address", suspectIP)
	}
	return fmt.Sprintf("%d.%d.%d.%d.%s", suspectIPv4[3], suspectIPv4[2], suspectIPv4[1], suspectIPv4[0], blLookupDomain), nil
}

// IsIPBlacklistIndication inspects the IP address resolved from blacklist and returns true only if the IP address
// is a positive indication of blacklisting, that is, the IP is in the range of 127.0.0.0/16.
func IsIPBlacklistIndication(ip net.IP) bool {
	_, local16Cidr, err := net.ParseCIDR("127.0.0.0/16")
	if err != nil {
		panic(err)
	}
	return local16Cidr.Contains(ip)
}

// IsSuspectIPBlacklisted looks up the suspect IP from all sources of spam blacklists. If the suspect IP is blacklisted by any
// of the spam blacklists, then the function will return the domain name of the DNSBL. If the suspect IP is not blacklisted or
// due to network error the blacklist status cannot be determined, then the function will return an empty string.
func IsSuspectIPBlacklisted(suspectIP string) string {
	// Wait for negative result from all look-up servers
	resultsAllIn := make(chan struct{})
	resultsWaitGroup := new(sync.WaitGroup)
	resultsWaitGroup.Add(len(SpamBlacklistLookupServers))
	// Collect individual lookup result within a second
	lookupResult := make(chan string, len(SpamBlacklistLookupServers))
	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), DNSBlackListQueryTimeoutSec*time.Second)
	defer timeoutCancel()
	for _, lookupDomain := range SpamBlacklistLookupServers {
		go func(lookupDomain string) {
			defer resultsWaitGroup.Done()
			lookupName, err := GetBlacklistLookupName(suspectIP, lookupDomain)
			if err != nil {
				// Cannot possibly blacklist an invalid suspect IP
				lookupResult <- ""
				return
			}
			// Validate the result to make sure it is a valid response to a DNS-based blacklist query
			ips, err := dnsd.NeutralRecursiveResolver.LookupIPAddr(timeoutCtx, lookupName)
			if err == nil {
				for _, ip := range ips {
					if IsIPBlacklistIndication(ip.IP) {
						lookupResult <- lookupDomain
						return
					}
				}
			}
		}(lookupDomain)
	}
	// Wait for negative result from all look-up servers
	go func() {
		resultsWaitGroup.Wait()
		close(resultsAllIn)
	}()
	select {
	case <-resultsAllIn:
		// All lookup servers came back with negative result
		return ""
	case <-timeoutCtx.Done():
		// None of the servers reached so far came back positive before timeout
		return ""
	case ret := <-lookupResult:
		// The domain name of DNSBL that responded with a positive indication, or empty string for invalid suspect IP.
		return ret
	}
}