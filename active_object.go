package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"log"

	"github.com/anon55555/mt/rudp"
)

const (
	AoCmdSetProps = iota
	AoCmdUpdatePos
	AoCmdSetTextureMod
	AoCmdSetSprite
	AoCmdPunched
	AoCmdUpdateArmorGroups
	AoCmdSetAnimation
	AoCmdSetBonePos
	AoCmdAttachTo
	AoCmdSetPhysicsOverride
	AoCmdObsolete1
	AoCmdSpawnInfant
	AoCmdSetAnimSpeed
)

func processAoRmAdd(c *Conn, r *bytes.Reader) []byte {
	w := &bytes.Buffer{}

	countRm := ReadUint16(r)
	WriteUint16(w, countRm)

	var aoRm []uint16
	for i := uint16(0); i < countRm; i++ {
		id := ReadUint16(r)

		if id == c.localPlayerCao {
			id = c.currentPlayerCao
		}

		WriteUint16(w, id)
		aoRm = append(aoRm, id)
	}

	countAdd := ReadUint16(r)
	WriteUint16(w, countAdd)

	var aoAdd []uint16
	for i := uint16(0); i < countAdd; i++ {
		id := ReadUint16(r)
		objType := ReadUint8(r)
		initData := ReadBytes32(r)

		dr := bytes.NewReader(initData)
		dr.Seek(1, io.SeekStart)

		name := string(ReadBytes16(dr))

		if name == c.Username() {
			if c.initAoReceived {
				// Read the messages from the packet
				// They need to be forwarded
				dr.Seek(30, io.SeekCurrent)

				msgcountByte, _ := dr.ReadByte()
				msgcount := uint8(msgcountByte)

				var msgs [][]byte
				for j := uint8(0); j < msgcount; j++ {
					dr.Seek(2, io.SeekCurrent)

					msglenBytes := make([]byte, 2)
					dr.Read(msglenBytes)
					msglen := binary.BigEndian.Uint16(msglenBytes)

					msg := make([]byte, msglen)
					dr.Read(msg)

					msgs = append(msgs, msg)
				}

				// Generate message packet
				msgpkt := []byte{0x00, ToClientActiveObjectMessages}
				for _, msg := range msgs {
					msgdata := make([]byte, 4+len(msg))
					binary.BigEndian.PutUint16(msgdata[0:2], c.localPlayerCao)
					binary.BigEndian.PutUint16(msgdata[2:4], uint16(len(msg)))
					copy(msgdata[4:], aoMsgReplaceIDs(c, msg))
					msgpkt = append(msgpkt, msgdata...)
				}

				ack, err := c.Send(rudp.Pkt{Reader: bytes.NewReader(msgpkt)})
				if err != nil {
					log.Print(err)
				}
				<-ack

				data := w.Bytes()
				binary.BigEndian.PutUint16(data[4+countRm*2:6+countRm*2], countAdd-1)
				w = bytes.NewBuffer(data)

				c.currentPlayerCao = id
				continue
			} else {
				c.initAoReceived = true
				c.localPlayerCao = id
				c.currentPlayerCao = id
			}
		} else if id == c.localPlayerCao {
			id = c.currentPlayerCao
		}

		if name != c.Username() {
			aoAdd = append(aoAdd, id)
		}

		WriteUint16(w, id)
		WriteUint8(w, objType)
		WriteBytes32(w, initData)
	}

	c.redirectMu.Lock()
	for i := range aoAdd {
		if aoAdd[i] != 0 {
			c.aoIDs[aoAdd[i]] = true
		}
	}

	for i := range aoRm {
		c.aoIDs[aoRm[i]] = false
	}
	c.redirectMu.Unlock()

	return w.Bytes()
}

func processAoMsgs(c *Conn, r *bytes.Reader) []byte {
	w := &bytes.Buffer{}

	for r.Len() >= 4 {
		id := ReadUint16(r)

		msg := aoMsgReplaceIDs(c, ReadBytes16(r))

		if id == c.currentPlayerCao {
			id = c.localPlayerCao
		} else if id == c.localPlayerCao {
			id = c.currentPlayerCao
		}

		WriteUint16(w, id)
		WriteBytes16(w, msg)
	}

	return w.Bytes()
}

func aoMsgReplaceIDs(c *Conn, data []byte) []byte {
	switch cmd := data[0]; cmd {
	case AoCmdAttachTo:
		id := binary.BigEndian.Uint16(data[1:3])
		if id == c.currentPlayerCao {
			id = c.localPlayerCao
			binary.BigEndian.PutUint16(data[1:3], id)
		} else if id == c.localPlayerCao {
			id = c.currentPlayerCao
			binary.BigEndian.PutUint16(data[1:3], id)
		}
	case AoCmdSpawnInfant:
		id := binary.BigEndian.Uint16(data[1:3])
		if id == c.currentPlayerCao {
			id = c.localPlayerCao
			binary.BigEndian.PutUint16(data[1:3], id)
		} else if id == c.localPlayerCao {
			id = c.currentPlayerCao
			binary.BigEndian.PutUint16(data[1:3], id)
		}
	}

	return data
}
