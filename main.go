package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"

	// agent "github.com/ltvco/ltv-apm-modules-go/agent"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type ImageInfo struct {
	LastPushed     time.Time
	LastSeen       time.Time
	Digest         string
	RegistryUrl    string
	RepositoryName string
	DeployedTag    string
	Tags           []string
	Cluster        string
	FullImagePath  string
}

type PostBody struct {
	From    string    `json:"from"`
	Queries []Queries `json:"queries"`
	To      string    `json:"to"`
}
type Datasource struct {
	UID string `json:"uid"`
}
type Queries struct {
	Datasource    Datasource `json:"datasource"`
	Format        string     `json:"format"`
	IntervalMs    int        `json:"intervalMs"`
	MaxDataPoints int        `json:"maxDataPoints"`
	Expr          string     `json:"Expr"`
	RefID         string     `json:"refId"`
}

type Response struct {
	Results struct {
		A struct {
			Status int `json:"status"`
			Frames []struct {
				Schema struct {
					RefID string `json:"refId"`
					Meta  struct {
						Type        string `json:"type"`
						TypeVersion []int  `json:"typeVersion"`
						Custom      struct {
							ResultType string `json:"resultType"`
						} `json:"custom"`
						ExecutedQueryString string `json:"executedQueryString"`
					} `json:"meta"`
					Fields []struct {
						Name     string `json:"name"`
						Type     string `json:"type"`
						TypeInfo struct {
							Frame string `json:"frame"`
						} `json:"typeInfo"`
						Config struct {
							Interval int `json:"interval"`
						} `json:"config,omitempty"`
						Labels struct {
							Name        string `json:"__name__"`
							Cluster     string `json:"cluster"`
							Container   string `json:"container"`
							ContainerID string `json:"container_id"`
							Endpoint    string `json:"endpoint"`
							Image       string `json:"image"`
							ImageID     string `json:"image_id"`
							ImageSpec   string `json:"image_spec"`
							Instance    string `json:"instance"`
							Job         string `json:"job"`
							Namespace   string `json:"namespace"`
							Pod         string `json:"pod"`
							Service     string `json:"service"`
							UID         string `json:"uid"`
						} `json:"labels,omitempty"`
					} `json:"fields"`
				} `json:"schema"`
				Data struct {
					Values [][]int64 `json:"values"`
				} `json:"data"`
			} `json:"frames"`
		} `json:"A"`
	} `json:"results"`
}

func main() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-east-1"), config.WithSharedConfigProfile("production"))
	if err != nil {
		log.Fatal().Err(err).Msg("unable to load SDK config")
	}

	allImages := GetPrometheusImagesFromProfile()

	allEcrImages, err := GetECRImages(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get ECR images")
		return
	}
	fmt.Printf("len(allImages): %v\n", len(allImages))
	for i, output := range allEcrImages {
		untagged := 0
		for _, deets := range output.ImageDetails {
			if deets.ImageTags == nil {
				untagged++
				continue
			}
			// fmt.Printf("Index: %v  imageName and tags: %v:%v\n", i, *deets.RepositoryName, deets.ImageTags)
			// for _, image := range allImages {

			// 	if *deets.RepositoryName == image.RepositoryName {
			// 		s, _ := json.MarshalIndent(deets, "", "\t")
			// 		fmt.Printf("image.RepositoryName: %v\n", image.RepositoryName)
			// 		fmt.Printf("s: %v\n", string(s))

			// 		image.Tags = deets.ImageTags
			// 		image.LastPushed = *deets.ImagePushedAt
			// 		image.Digest = *deets.ImageDigest

			// 	}
			// }
		}
		fmt.Printf("untagged in %v: %v\n", i, untagged)
	}
	fmt.Printf("before unique allImages: %v\n", len(allImages))
	allImages = GetUnique(allImages)
	fmt.Printf("after unique allImages: %v\n", len(allImages))

	for _, image := range allImages {
		_, err := json.MarshalIndent(image, "", "\t")
		if err != nil {
			log.Error().Err(err).Msg("failed to marshalIndent json")
		}
		// fmt.Printf("s: %v\n", string(s))

		// if image.LastPushed.Before(time.Now().AddDate(0, 0, -(daysOld))) {
		// 	fmt.Printf("%v is older than %v days old\n", image.FullImagePath, daysOld)
		// }
		for _, ecr := range allEcrImages {
			for _, deets := range ecr.ImageDetails {
				// fmt.Printf("deets.ImageDigest: %v\n", *deets.ImageDigest)
				deetsDigest := strings.Split(*deets.ImageDigest, ":")[1]
				// fmt.Printf("deetsDigest: %v\n", deetsDigest)
				// fmt.Printf("image.Digest: %v\n", image.Digest)
				if deetsDigest == image.Digest {

					fmt.Printf("running image %v:%v pushed at %v\n", *deets.RepositoryName, deets.ImageTags, deets.ImagePushedAt)
					continue
				}
			}
		}
	}

}

func GetPrometheusImagesFromProfile() []*ImageInfo {
	allImages := make([]*ImageInfo, 0)
	datasource := Datasource{
		UID: "W7Xb02Snk",
	}
	query := Queries{
		RefID: "A",
		Expr:  "kube_pod_container_info{cluster=\"promoted-walleye\"}",

		Datasource: datasource,
	}
	postBody := PostBody{
		From:    "now-1h",
		Queries: []Queries{query},
		To:      "now",
	}
	buf, err := json.Marshal(postBody)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshall json")
	}
	grafanaApiKey := os.Getenv("GRAFANA_API_KEY")
	req, _ := http.NewRequest("POST", "https://grafana.ltvops.com/api/ds/query", bytes.NewBuffer(buf))
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", grafanaApiKey))
	req.Header.Add("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("failed to call grafana")
	}
	defer resp.Body.Close()

	body := &Response{}
	json.NewDecoder(resp.Body).Decode(body)
	for _, item := range body.Results.A.Frames {

		for _, field := range item.Schema.Fields {
			if field.Labels.Name == "kube_pod_container_info" {

				// var lastSeen time.Time
				// if item.Data.Values[1][0] != 0 {
				// 	lastSeen = time.Unix(item.Data.Values[1][0], 0)
				// }

				registryUrl := strings.Split(strings.Split(field.Labels.Image, "@")[0], ":")[0]
				if field.Labels.ImageID != "" {
					// fmt.Printf("field.Labels.ImageID: %v\n", field.Labels.ImageID)
					digest := strings.Split(strings.Split(field.Labels.ImageID, "@")[1], ":")[1]
					newImage := ImageInfo{
						FullImagePath:  field.Labels.Image,
						RegistryUrl:    registryUrl,
						RepositoryName: strings.Split(registryUrl, "/")[len(strings.Split(registryUrl, "/"))-1],
						DeployedTag:    strings.Split(field.Labels.Image, ":")[1],
						Digest:         digest,
						Cluster:        field.Labels.Cluster,
					}
					allImages = append(allImages, &newImage)
				}

			}
		}
	}
	return allImages
}

func GetECRImages(cfg aws.Config) ([]*ecr.DescribeImagesOutput, error) {
	var allECRImages []*ecr.DescribeImagesOutput
	client := ecr.NewFromConfig(cfg)
	registry, _ := client.DescribeRegistry(context.Background(), &ecr.DescribeRegistryInput{})
	fmt.Printf("repos.RegistryId: %v\n", *registry.RegistryId)
	repos, _ := client.DescribeRepositories(context.Background(), &ecr.DescribeRepositoriesInput{})
	for _, repo := range repos.Repositories {
		if repo.RepositoryName != nil {

			input := &ecr.ListImagesInput{
				RepositoryName: repo.RepositoryName,
			}

			images, err := client.ListImages(context.Background(), input)
			if err != nil {
				log.Error().Err(err).Msg("failed to list images")
				return nil, err
			}
			if len(images.ImageIds) > 0 {

				describe, err := client.DescribeImages(context.Background(), &ecr.DescribeImagesInput{
					ImageIds:       images.ImageIds,
					RepositoryName: repo.RepositoryName,
				})
				if err != nil {
					log.Error().Err(err).Msg("failed to describe images")
					return nil, err
				}
				allECRImages = append(allECRImages, describe)

			}

		}
	}
	return allECRImages, nil
}

func GetUnique(all []*ImageInfo) []*ImageInfo {
	var unique []*ImageInfo
	for _, v := range all {
		skip := false
		for _, u := range unique {
			if v.FullImagePath == u.FullImagePath {
				skip = true
				break
			}
		}
		if !skip {
			unique = append(unique, v)
		}
	}
	return unique
}
