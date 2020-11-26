package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	wires "github.com/huiscool/topic-wires"
	"github.com/libp2p/go-libp2p"
	peer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p/config"
	"github.com/multiformats/go-multiaddr"
)

func main() {
	var rendezvous *string
	rendezvous = flag.String("h", "", "rendezvous point")
	flag.Parse()

	var conf []config.Option
	if *rendezvous == "" {
		conf = append(conf, libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/6666"))
	}

	h, err := libp2p.New(
		context.Background(),
		conf...,
	)
	if err != nil {
		panic(err)
	}

	w, err := wires.NewTopicWires(h)
	if err != nil {
		panic(err)
	}

	w.SetNotifiee(&wires.NotifAdapter{
		HandleUp: func(neigh peer.ID, topic string) {
			fmt.Printf("%s: peer %s has joined %s",
				h.ID().ShortString(), neigh.ShortString(), topic)
		},
		HandleDown: func(neigh peer.ID, topic string) {
			fmt.Printf("%s: peer %s has lefted %s",
				h.ID().ShortString(), neigh.ShortString(), topic)
		},
	})

	if *rendezvous == "" {
		fmt.Println("open another console and enter:")
		fmt.Printf("go run example/example.go -h /ip4/127.0.0.1/tcp/6666/p2p/%s\n", h.ID().Pretty())
		fmt.Println("usage:")
		fmt.Println("JOIN <topic>")
		fmt.Println("LEAVE <topic>")
		fmt.Println("PEERS")
		fmt.Println("waiting for incoming conections ...")
		CLI(w)
	}

	// connetct to rendezvous point
	a, err := multiaddr.NewMultiaddr(*rendezvous)
	if err != nil {
		panic(err)
	}
	info, err := peer.AddrInfoFromP2pAddr(a)
	if err != nil {
		panic(err)
	}
	err = h.Connect(context.Background(), *info)
	if err != nil {
		panic(err)
	}
	fmt.Println("rendezvous connected")
	CLI(w)
}

func CLI(w *wires.TopicWires) {
	stdReader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("> ")
		sendData, err := stdReader.ReadString('\n')

		if err != nil {
			panic(err)
		}
		if strings.HasPrefix(sendData, "PEERS") {
			fmt.Println(w.Topics())
			for _, topic := range w.Topics() {
				fmt.Println(w.Neighbors(topic))
			}
		}
		if strings.HasPrefix(sendData, "JOIN ") {
			err = w.Join(strings.TrimPrefix(sendData, "JOIN "))
		}
		if strings.HasPrefix(sendData, "LEAVE ") {
			err = w.Leave(strings.TrimPrefix(sendData, "LEAVE "))
		}
		if err != nil {
			panic(err)
		}
	}

}
