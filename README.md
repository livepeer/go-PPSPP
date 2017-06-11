# go-PPSPP
Go implementation of the Peer-to-Peer Streaming Peer Protocol ([rfc7574](https://tools.ietf.org/html/rfc7574))

This protocol serves as a transport layer for video live streaming.  It is format-agnostic (can be used to transport any video format), and network protocol-agnostic (should be able to talk to any network protocol like tcp, udp, webrtc, etc)

### Overall Operation (Taken from rfc7574 section 2)

##### 1: Joining a Swarm

Consider a user who wants to watch a video.  To play the video, the
user clicks on the play button of a HTML5 <video> element shown in
his PPSPP-enabled browser.  Imagine this element has a PPSPP URL (to
be defined elsewhere) identifying the video as its source.  The
browser passes this URL to its peer-to-peer streaming protocol
handler.  Let's call this protocol handler Peer A.  Peer A parses the
URL to retrieve the transport address of a peer-to-peer streaming
protocol tracker and swarm metadata of the content.  The tracker
address may be optional in the presence of a decentralized tracking
mechanism.  The mechanisms for tracking peers are outside of the
scope of this document.

Peer A now registers with the tracker following the peer-to-peer
streaming protocol tracker specification [PPSP-TP] and receives the
IP address and port of peers already in the swarm, say, Peers B, C,
and D.  At this point, the PPSPP starts operating.  Peer A now sends
a datagram containing a PPSPP HANDSHAKE message to Peers B, C, and D.
This message conveys protocol options.  In particular, Peer A
includes the ID of the swarm (part of the swarm metadata) as a
protocol option because the destination peers can listen for multiple
swarms on the same transport address.

Peers B and C respond with datagrams containing a PPSPP HANDSHAKE
message and one or more HAVE messages.  A HAVE message conveys (part
of) the chunk availability of a peer; thus, it contains a chunk
specification that denotes what chunks of the content Peers B and C
have, respectively.  Peer D sends a datagram with a HANDSHAKE and
HAVE messages, but also with a CHOKE message.  The latter indicates
that Peer D is not willing to upload chunks to Peer A at present.

##### 2: Exchanging Chunks

In response to Peers B and C, Peer A sends new datagrams to Peers B
and C containing REQUEST messages.  A REQUEST message indicates the
chunks that a peer wants to download; thus, it contains a chunk
specification.  The REQUEST messages to Peers B and C refer to
disjoint sets of chunks.  Peers B and C respond with datagrams
containing HAVE, DATA, and, in this example, INTEGRITY messages.  In
the Merkle hash tree content protection scheme (see Section 5.1), the
INTEGRITY messages contain all cryptographic hashes that Peer A needs
to verify the integrity of the content chunk sent in the DATA
message.  Using these hashes, Peer A verifies that the chunks
received from Peers B and C are correct against the trusted swarm ID.
Peer A also updates the chunk availability of Peers B and C using the
information in the received HAVE messages.  In addition, it passes
the chunks of video to the user's browser for rendering.

After processing, Peer A sends a datagram containing HAVE messages
for the chunks it just received to all its peers.  In the datagram to
Peers B and C, it includes an ACK message acknowledging the receipt
of the chunks and adds REQUEST messages for new chunks.  ACK messages
are not used when a reliable transport protocol is used.  When, for
example, Peer C finds that Peer A obtained a chunk (from Peer B) that
Peer C did not yet have, Peer C's next datagram includes a REQUEST
for that chunk.

Peer D also sends HAVE messages to Peer A when it downloads chunks
from other peers.  When Peer D is willing to accept REQUESTs from
Peer A, Peer D sends a datagram with an UNCHOKE message to inform
Peer A.  If Peer B or C decides to choke Peer A, they send a CHOKE
message and Peer A should then re-request from other peers.  Peers B
and C may continue to send HAVE, REQUEST, or periodic keep-alive
messages such that Peer A keeps sending them HAVE messages.

Once Peer A has received all content (video-on-demand use case), it
stops sending messages to all other peers that have all content
(a.k.a. seeders).  Peer A can also contact the tracker or another
source again to obtain more peer addresses.

##### 3: Leaving a Swarm

To leave a swarm in a graceful way, Peer A sends a specific HANDSHAKE
message to all its peers (see Section 8.4) and deregisters from the
tracker following the tracker specification [PPSP-TP].  Peers
receiving the datagram should remove Peer A from their current peer
list.  If Peer A crashes ungracefully, peers should remove Peer A
from their peer list when they detect it no longer sends messages
(see Section 3.12).
