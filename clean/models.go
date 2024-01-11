package clean

import "time"

type ImageInfo struct {
	LastPushed     time.Time `json:"last_pushed,omitempty"`
	LastSeen       time.Time `json:"last_seen,omitempty"`
	Digest         string    `json:"digest,omitempty"`
	RegistryUrl    string    `json:"registry_url,omitempty"`
	RepositoryName string    `json:"repository_name,omitempty"`
	DeployedTag    string    `json:"deployed_tag,omitempty"`
	Tags           []string  `json:"tags,omitempty"`
	Cluster        string    `json:"cluster,omitempty"`
	FullImagePath  string    `json:"full_image_path,omitempty"`
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
