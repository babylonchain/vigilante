package e2etest

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func baseDir() (string, error) {
	dirPath := filepath.Join(os.TempDir(), "btcdwallet", "rpctest")
	err := os.MkdirAll(dirPath, 0755)
	return dirPath, err
}

type wallet struct {
	cmd     *exec.Cmd
	pidFile string
	dataDir string
}

func newWallet(dataDir string, cmd *exec.Cmd) *wallet {
	return &wallet{
		dataDir: dataDir,
		cmd:     cmd,
	}
}

func (n *wallet) start() error {
	if err := n.cmd.Start(); err != nil {
		return err
	}

	pid, err := os.Create(filepath.Join(n.dataDir,
		fmt.Sprintf("%s.pid", "config")))
	if err != nil {
		return err
	}

	n.pidFile = pid.Name()
	if _, err = fmt.Fprintf(pid, "%d\n", n.cmd.Process.Pid); err != nil {
		return err
	}

	if err := pid.Close(); err != nil {
		return err
	}

	return nil
}

func (n *wallet) stop() (err error) {
	if n.cmd == nil || n.cmd.Process == nil {
		// return if not properly initialized
		// or error starting the process
		return nil
	}

	defer func() {
		err = n.cmd.Wait()
	}()

	if runtime.GOOS == "windows" {
		return n.cmd.Process.Signal(os.Kill)
	}
	return n.cmd.Process.Signal(os.Interrupt)
}

func (n *wallet) cleanup() error {
	if n.pidFile != "" {
		if err := os.Remove(n.pidFile); err != nil {
			log.Printf("unable to remove file %s: %v", n.pidFile,
				err)
		}
	}

	dirs := []string{
		n.dataDir,
	}
	var err error
	for _, dir := range dirs {
		if err = os.RemoveAll(dir); err != nil {
			log.Printf("Cannot remove dir %s: %v", dir, err)
		}
	}
	return err
}

func (n *wallet) shutdown() error {
	if err := n.stop(); err != nil {
		return err
	}
	if err := n.cleanup(); err != nil {
		return err
	}
	return nil
}

type WalletHandler struct {
	wallet *wallet
}

func NewWalletHandler(btcdCert []byte, walletPath string, btcdHost string) (*WalletHandler, error) {
	testDir, err := baseDir()
	logsPath := filepath.Join(testDir, "logs")
	walletDir := filepath.Join(testDir, "simnet")

	if err := os.Mkdir(walletDir, os.ModePerm); err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	certFile := filepath.Join(testDir, "rpc.cert")

	dir := fmt.Sprintf("--appdata=%s", testDir)
	logDir := fmt.Sprintf("--logdir=%s", logsPath)
	certDir := fmt.Sprintf("--cafile=%s", certFile)
	hostConf := fmt.Sprintf("--rpcconnect=%s", btcdHost)

	// Write cert and key files.
	if err = os.WriteFile(certFile, btcdCert, 0666); err != nil {
		return nil, err
	}

	tempWalletPath := filepath.Join(walletDir, "wallet.db")

	// Read all content of src to data, may cause OOM for a large file.
	data, err := os.ReadFile(walletPath)

	if err != nil {
		return nil, err
	}

	// Write data to dst
	err = os.WriteFile(tempWalletPath, data, 0644)

	if err != nil {
		return nil, err
	}

	// btcwallet --btcdusername=user --btcdpassword=pass --username=user --password=pass --noinitialload --noservertls --simnet
	createCmd := exec.Command(
		"btcwallet",
		"--debuglevel=debug",
		"--btcdusername=user",
		"--btcdpassword=pass",
		"--username=user",
		"--password=pass",
		"--noservertls",
		"--simnet",
		hostConf,
		certDir,
		dir,
		logDir,
	)

	return &WalletHandler{
		wallet: newWallet(testDir, createCmd),
	}, nil
}

func (w *WalletHandler) Start() error {
	if err := w.wallet.start(); err != nil {
		return w.wallet.cleanup()
	}
	return nil
}

func (w *WalletHandler) Stop() error {
	if err := w.wallet.shutdown(); err != nil {
		return err
	}

	return nil
}
