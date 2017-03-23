package dnsd

import (
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"
)

// Send forward queries to forwarder and forward the response to my DNS client.
func (dnsd *DNSD) ForwarderQueueProcessor(myQueue chan *UDPForwarderQuery, forwarderConn net.Conn) {
	packetBuf := make([]byte, MaxPacketSize)
	for {
		query := <-myQueue
		// Set deadline for IO with forwarder
		forwarderConn.SetDeadline(time.Now().Add(IOTimeoutSec * time.Second))
		if _, err := forwarderConn.Write(query.QueryPacket); err != nil {
			dnsd.Logger.Printf("ForwarderQueueProcessor", "Write", err, "IO failure")
			continue
		}
		packetLength, err := forwarderConn.Read(packetBuf)
		if err != nil {
			dnsd.Logger.Printf("ForwarderQueueProcessor", "Read", err, "IO failure")
			continue
		}
		// Set deadline for responding to my DNS client
		query.MyServer.SetWriteDeadline(time.Now().Add(IOTimeoutSec * time.Second))
		if _, err := query.MyServer.WriteTo(packetBuf[:packetLength], query.ClientAddr); err != nil {
			dnsd.Logger.Printf("ForwarderQueueProcessor", "WriteResponse", err, "IO failure")
			continue
		}
		dnsd.Logger.Printf("ForwarderQueueProcessor", query.ClientAddr.IP.String(), nil,
			"successfully forwarded answer for \"%s\", backlog length %d", query.DomainName, len(myQueue))
	}
}

/*
You may call this function only after having called Initialise()!
Start DNS daemon to listen on UDP port only. Block caller.
*/
func (dnsd *DNSD) StartAndBlockUDP() error {
	listenAddr := fmt.Sprintf("%s:%d", dnsd.UDPListenAddress, dnsd.UDPListenPort)
	udpAddr, err := net.ResolveUDPAddr("udp", listenAddr)
	if err != nil {
		return err
	}
	udpServer, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}
	// Start forwarder queues that will forward client queries and respond to them
	for i, queue := range dnsd.UDPForwarderQueues {
		go dnsd.ForwarderQueueProcessor(queue, dnsd.UDPForwarderConns[i])
	}
	// Dispatch queries to forwarder queues
	packetBuf := make([]byte, MaxPacketSize)
	dnsd.Logger.Printf("StartAndBlockUDP", listenAddr, nil, "going to listen for queries")
	for {
		packetLength, clientAddr, err := udpServer.ReadFromUDP(packetBuf)
		if err != nil {
			return err
		}
		// Check address against rate limit
		clientIP := clientAddr.IP.String()
		if !dnsd.RateLimit.Add(clientIP, true) {
			continue
		}
		// Check address against allowed IP prefixes
		var prefixOK bool
		for _, prefix := range dnsd.AllowQueryIPPrefixes {
			if strings.HasPrefix(clientIP, prefix) {
				prefixOK = true
				break
			}
		}
		if !prefixOK {
			dnsd.Logger.Printf("UDPLoop", clientIP, nil, "client IP is not allowed by configuration")
			continue
		}

		// Prepare parameters for forwarding the query
		randForwarder := rand.Intn(len(dnsd.UDPForwarderQueues))
		forwardPacket := make([]byte, packetLength)
		copy(forwardPacket, packetBuf[:packetLength])
		domainName := ExtractDomainName(forwardPacket)
		if domainName == "" {
			// If I cannot figure out what domain is from the query, simply forward it without much concern.
			dnsd.Logger.Printf("UDPLoop", clientIP, nil, "let forwarder %d handle non-name query", randForwarder)

		} else {
			// This is a domain name query, check the name against black list and then forward.
			dnsd.BlackListMutex.Lock()
			_, blacklisted := dnsd.BlackList[domainName]
			dnsd.BlackListMutex.Unlock()
			if blacklisted {
				dnsd.Logger.Printf("UDPLoop", clientIP, nil, "answer to black-listed domain \"%s\"", domainName)
				blackHoleAnswer := RespondWith0(forwardPacket)
				udpServer.SetWriteDeadline(time.Now().Add(IOTimeoutSec * time.Second))
				if _, err := udpServer.WriteTo(blackHoleAnswer, clientAddr); err != nil {
					dnsd.Logger.Printf("UDPLoop", clientAddr.IP.String(), err, "IO failure")
				}

				continue
			} else {
				dnsd.Logger.Printf("UDPLoop", clientIP, nil, "let forwarder %d handle domain \"%s\"", randForwarder, domainName)
				// Forwarder queue will take care of this query
			}
		}
		dnsd.UDPForwarderQueues[randForwarder] <- &UDPForwarderQuery{
			ClientAddr:  clientAddr,
			DomainName:  domainName,
			MyServer:    udpServer,
			QueryPacket: forwardPacket,
		}
	}
}
