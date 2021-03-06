package factory

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
)

type RKT struct {
	name string
	ID   string
}

func (this *RKT) init() {
	this.name = ""
	this.ID = ""
}

func (this *RKT) SetRT(runtime string) {
	this.name = "rkt"
}

func (this *RKT) GetRT() string {
	return "rkt"
}

func (this *RKT) GetRTID() string {
	return this.ID
}

func (this *RKT) Convert(appName string, workingDir string) (string, error) {
	var cmd *exec.Cmd
	aciName := appName + ".aci"
	//set appName to rkt appname, set rkt aciName to image name
	cmd = exec.Command("../plugins/oci2aci", "--debug", "--name", appName, appName, aciName)
	cmd.Dir = workingDir
	cmd.Stdin = os.Stdin

	out, err := cmd.CombinedOutput()

	logrus.Debugf("Command done")
	if err != nil {
		return string(out), errors.New(string(out) + err.Error())
	}

	return string(out), nil
}

func (this *RKT) StartRT(specDir string) (string, error) {
	logrus.Debugf("Launcing runtime")

	appName := filepath.Base(specDir)
	aciName := appName + ".aci"
	aciPath := filepath.Dir(specDir)

	if retStr, err := this.Convert(appName, aciPath); err != nil {
		return retStr, err
	}

	cmd := exec.Command("rkt", "run", aciName, "--interactive", "--insecure-skip-verify", "--mds"+
		"-register=false", "--volume", "proc,kind=host,source=/bin", "--volume", "dev,kind=host,"+
		"source=/bin", "--volume", "devpts,kind=host,source=/bin", "--volume", "shm,kind=host,"+
		"source=/bin", "--volume", "mqueue,kind=host,source=/bin", "--volume", "sysfs,kind=host,"+
		"source=/bin", "--volume", "cgroup,kind=host,source=/bin", "--net=host")
	cmd.Dir = aciPath
	cmd.Stdin = os.Stdin
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), err
	}
	logrus.Debugf("Command done")

	id, bv, ev := checkResult(appName)
	this.ID = id
	if ev != nil {
		return "", ev
	} else if !bv {
		return string(out), errors.New(string(out))
	}
	return string(out), nil
}

func checkResult(appName string) (string, bool, error) {

	//use rkt list to get uuid of rkt contianer
	cmd := exec.Command("rkt", "list")
	cmd.Stdin = os.Stdin
	listOut, err := cmd.CombinedOutput()
	if err != nil {
		logrus.Fatalf("rkt list err %v\n", err)
	}

	uuid, err := getUuid(string(listOut), appName)
	if err != nil {
		return "", false, errors.New("can not get uuid of rkt app" + appName)
	}
	logrus.Debugf("uuid: %v\n", uuid)

	//use rkt status to get status of app running in rkt container
	cmd = exec.Command("rkt", "status", uuid)
	statusOut, err := cmd.CombinedOutput()
	if err != nil {
		logrus.Fatalf("rkt status err %v\n", err)
	}
	logrus.Debugf("rkt stauts %v\n,%v\n", uuid, string(statusOut))

	s, err := getAppStatus(string(statusOut), appName)
	if s != 0 || err != nil {
		return uuid, false, err
	}

	return uuid, true, nil
}

func getAppStatus(Out string, appName string) (int64, error) {
	line, err := getLine(Out, appName)
	if err != nil {
		logrus.Debugln(err)
		return 1, err
	}
	a := strings.SplitAfter(line, "=")

	res, err := strconv.ParseInt(a[1], 10, 32)
	if err != nil {
		logrus.Debugln(err)
		return 1, err
	}
	return res, nil
}

func getUuid(listOut string, appName string) (string, error) {

	line, err := getLine(listOut, appName)
	if err != nil {
		logrus.Debugln(err)
		return "", err
	}

	return splitUuid(line), nil
}

func splitUuid(line string) string {

	a := strings.Fields(line)
	return strings.TrimSpace(a[0])
}

func getLine(Out string, objName string) (string, error) {

	outArray := strings.Split(Out, "\n")
	flag := false
	var wantLine string
	for _, o := range outArray {
		if strings.Contains(o, objName) {
			wantLine = o
			flag = true
			break
		}
	}

	if !flag {
		return wantLine, errors.New("no line containers " + objName)
	}
	return wantLine, nil
}

func (this *RKT) StopRT(id string) error {

	cmd := exec.Command("rkt", "rm", id)
	cmd.Stdin = os.Stdin
	_, _ = cmd.CombinedOutput()

	return nil
}
