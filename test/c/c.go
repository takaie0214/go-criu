package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"log"
	rpc2 "net/rpc"

	"github.com/checkpoint-restore/go-criu"
	"github.com/checkpoint-restore/go-criu/phaul"
	"github.com/checkpoint-restore/go-criu/rpc"
	"github.com/golang/protobuf/proto"
)

const (
	address = "0.0.0.0"
	rpc_port = ":1234"
	pageserver_port = 5678
)

type testLocal struct {
	criu.NoNotify
	r *testRemote
//	r *rpc2.Client
}

type testRemote struct {
	cli *rpc2.Client
}

/* Dir where test will put dump images */
const imagesDir = "image"

func prepareImages() error {
	err := os.Mkdir(imagesDir, 0700)
	if err != nil {
		return err
	}

	/* Work dir for PhaulClient */
	err = os.Mkdir(imagesDir+"/local", 0700)
	if err != nil {
		return err
	}

	/* Work dir for PhaulServer */
	err = os.Mkdir(imagesDir+"/remote", 0700)
	if err != nil {
		return err
	}

	/* Work dir for DumpCopyRestore */
	err = os.Mkdir(imagesDir+"/test", 0700)
	if err != nil {
		return err
	}

	return nil
}

type Args struct {
}
func (l *testLocal) PostDump() error {
	fmt.Printf("postdump stage\n")
	scpCmd := "sudo /usr/bin/scp -r  image/test/* root@" + address + ":/tmp/livemig"

	cmd := exec.Command("/bin/sh", "-c", scpCmd)

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("scp failed")
		fmt.Println(cmd)
		fmt.Println(string(output))
		return err
	}


	var reply int
	args := &Args{}

	err = l.r.cli.Call("Srvapi.DoRestore", args, &reply)
	if err != nil {
		return err
	}

	fmt.Printf("fin postdump\n")
	return nil
}

func (l *testLocal) DumpCopyRestore(cr *criu.Criu, cfg phaul.Config, lastClnImagesDir string) error {
	fmt.Printf("Final stage\n")

	imgDir, err := os.Open(imagesDir + "/test")
	if err != nil {
		return err
	}
	defer imgDir.Close()

	psi := rpc.CriuPageServerInfo{
		//Fd: proto.Int32(int32(cfg.Memfd)),
		Address: proto.String(cfg.Addr),
		Port:    proto.Int32(int32(cfg.Port)),
	}

	opts := rpc.CriuOpts{
		Pid:         proto.Int32(int32(cfg.Pid)),
		LogLevel:    proto.Int32(4),
		LogFile:     proto.String("dump.log"),
		ImagesDirFd: proto.Int32(int32(imgDir.Fd())),
		TrackMem:    proto.Bool(true),
		ParentImg:   proto.String(lastClnImagesDir),
		Ps:          &psi,
		//ShellJob:    proto.Bool(true),
	}

	fmt.Printf("Do dump\n")
	return cr.Dump(opts, l)
}

func (r *testRemote) StartIter() error {
	fmt.Printf("StartIter!\n")

	var reply int
	args := &Args{}

	err := r.cli.Call("Srvapi.StartIter", args, &reply)
	if err != nil{
		log.Fatalf("call err:",err)
		return err
	}
	return nil
}

func (r *testRemote) StopIter() error {
	fmt.Printf("StopIter!\n")

	var reply int
	args := &Args{}

	err := r.cli.Call("Srvapi.StopIter", args, &reply)

	if err != nil{
		log.Fatalf("call err:",err)
		return err
	}
	return nil
}


func main() {
	pid, _ := strconv.Atoi(os.Args[1])

	err := prepareImages()
	if err != nil {
		fmt.Printf("Can't prepare dirs for images: %v\n", err)
		os.Exit(1)
		return
	}

	client, err := rpc2.DialHTTP("tcp", address + rpc_port)
	if err != nil {
		log.Fatalf("dial err:",err)
	}

	r := &testRemote{client}

	cln, err := phaul.MakePhaulClient(&testLocal{r: r}, r,
		phaul.Config{
			Pid:   pid,
			Addr:  address,
			Port:  pageserver_port,
			Wdir:  imagesDir + "/local"})

	if err != nil {
		fmt.Printf("Unable to run a client: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Migrate\n")
	err = cln.Migrate()
	if err != nil {
		fmt.Printf("Failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("SUCCESS!\n")
}
