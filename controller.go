/*
Copyright 2015 The Kubernetes Authors All rights reserved.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for  the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"log"
	"os"
	"os/exec"
	"reflect"
	"text/template"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/api/unversioned"
)

const (
	nginxConf = `
events {
  worker_connections 1024;
}
http {
{{range $ing := .Items}}
{{range $rule := $ing.Spec.Rules}}
  server {
    listen 80;
    server_name {{$rule.Host}};
    resolver 127.0.0.1;
{{ range $path := $rule.HTTP.Paths }}
    {{if eq $path.Path "" }}
    location / {
    {{else}}
    location {{$path.Path}} {
    {{end}}
      proxy_pass http://{{$path.Backend.ServiceName}}:{{$path.Backend.ServicePort}}/;
      proxy_set_header Host $host;
    }{{end}}
  }{{end}}{{end}}
}`

//nginxConf = `
//events {
//  worker_connections 1024;
//}
//http {
//{{range $ing := .Items}}
//{{range $rule := $ing.Spec.Rules}}
//  server {
//    listen 80;
//    server_name {{$rule.Host}};
//    resolver 127.0.0.1;
//    proxy_http_version 1.1;
//	proxy_set_header HOST $host;
//	proxy_set_header X-Forwarded-Proto $scheme;
//	proxy_set_header X-Real-IP $remote_addr;
//	proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
//{{ range $path := $rule.HTTP.Paths }}
//    {{if eq $path.Path "" }}
//    location / {
//    {{else}}
//    location {{$path.Path}} {
//    {{end}}
//      proxy_pass http://{{$path.Backend.ServiceName}}:{{$path.Backend.ServicePort}}/;
//      proxy_set_header Upgrade $http_upgrade;
//	  proxy_set_header Connection 'upgrade';
//	  proxy_cache_bypass $http_upgrade;
//    }{{end}}
//  }{{end}}{{end}}
//}`
)

func shellOut(cmd string) {
	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()

	log.Println(" cmd ", cmd, string(out))

	if err != nil {
		log.Fatalf("Failed to execute %v: %v, err: %v", cmd, string(out), err)
	}
}

func main() {
	log.SetFlags(log.Flags()|log.Lshortfile)

	var ingClient client.IngressInterface
	if kubeClient, err := client.NewInCluster(); err != nil {
		log.Fatalf("Failed to create client: %v.", err)
	} else {
		ingClient = kubeClient.Extensions().Ingress(api.NamespaceAll)
	}
	tmpl, _ := template.New("nginx").Parse(nginxConf)
	rateLimiter := util.NewTokenBucketRateLimiter(0.1, 1)
	known := &extensions.IngressList{}

	log.Println("Start nginx...")
	// Controller loop
	shellOut("nginx")
	log.Println("Nginx start success")

	for {
		rateLimiter.Accept()
		options := unversioned.ListOptions{
			LabelSelector: unversioned.LabelSelector{labels.Everything()},
			FieldSelector: unversioned.FieldSelector{fields.Everything()},
		}

		ingresses, err := ingClient.List(options)
		log.Println("err :", err.Error())
		if err != nil || reflect.DeepEqual(ingresses.Items, known.Items) {
			continue
		}

		known = ingresses
		if w, err := os.Create("/etc/nginx/nginx.conf"); err != nil {
			log.Fatalf("Failed to open %v: %v", nginxConf, err)
		} else if err := tmpl.Execute(w, ingresses); err != nil {
			log.Fatalf("Failed to write template %v", err)
		}

		log.Println("Reload nginx")
		shellOut("nginx -s reload")
	}
}