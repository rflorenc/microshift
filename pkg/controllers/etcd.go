/*
Copyright © 2021 Microshift Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package controllers

import (
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	etcd "go.etcd.io/etcd/embed"

	"github.com/openshift/microshift/pkg/util"
)

const (
	etcdStartupTimeout = 10
)

func StartEtcd(ready chan bool) error {
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed to get hostname: %v", err)
	}
	ip, err := util.GetHostIP()
	if err != nil {
		return fmt.Errorf("failed to get host IP: %v", err)
	}
	// based on https://github.com/openshift/cluster-etcd-operator/blob/master/bindata/bootkube/bootstrap-manifests/etcd-member-pod.yaml#L19
	cfg := etcd.NewConfig()
	cfg.Logger = "zap"
	cfg.Dir = "/var/lib/etcd/"
	cfg.APUrls = setURL([]string{ip}, ":2380")
	cfg.LPUrls = setURL([]string{"0.0.0.0"}, ":2380")
	cfg.ACUrls = setURL([]string{ip}, ":2379")
	cfg.LCUrls = setURL([]string{"0.0.0.0"}, ":2379")
	cfg.Name = hostname
	cfg.InitialCluster = "default=https://0.0.0.0:2380," + hostname + "=" + "https://" + ip + ":2380"

	cfg.ClientTLSInfo.CertFile = "/etc/kubernetes/static-pod-certs/secrets/etcd-all-serving/" + "etcd-serving-" + hostname + ".crt"
	cfg.ClientTLSInfo.KeyFile = "/etc/kubernetes/static-pod-certs/secrets/etcd-all-serving/" + "etcd-serving-" + hostname + ".key"
	cfg.ClientTLSInfo.TrustedCAFile = "/etc/kubernetes/static-pod-certs/configmaps/etcd-serving-ca/ca-bundle.crt"
	cfg.ClientTLSInfo.ClientCertAuth = true
	cfg.ClientTLSInfo.InsecureSkipVerify = false //TODO

	cfg.PeerTLSInfo.CertFile = "/etc/kubernetes/static-pod-certs/secrets/etcd-all-peer/" + "etcd-peer-" + hostname + ".crt"
	cfg.PeerTLSInfo.KeyFile = "/etc/kubernetes/static-pod-certs/secrets/etcd-all-peer/" + "etcd-peer-" + hostname + ".key"
	cfg.PeerTLSInfo.TrustedCAFile = "/etc/kubernetes/static-pod-certs/configmaps/etcd-peer-client-ca/ca-bundle.crt"
	cfg.PeerTLSInfo.ClientCertAuth = true
	cfg.PeerTLSInfo.InsecureSkipVerify = false //TODO

	e, err := etcd.StartEtcd(cfg)
	if err != nil {
		return fmt.Errorf("etcd failed to start: %v", err)
	}
	go func() {
		select {
		case <-e.Server.ReadyNotify():
			logrus.Info("Server is ready!")
			ready <- true
		case <-time.After(etcdStartupTimeout * time.Second):
			e.Server.Stop()
			logrus.Fatalf("etcd failed to start in %d seconds", etcdStartupTimeout)
		}
	}()
	return nil
}

func setURL(hostnames []string, port string) []url.URL {
	urls := make([]url.URL, len(hostnames))
	for i, name := range hostnames {
		u, err := url.Parse("https://" + name + port)
		if err != nil {
			return []url.URL{}
		}
		urls[i] = *u
	}
	return urls
}