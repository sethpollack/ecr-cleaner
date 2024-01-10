package main

import (

	// agent "github.com/ltvco/ltv-apm-modules-go/agent"

	"os"
	"strconv"

	"github.com/ltvco/ecr-cleaner/cmd"
	"github.com/rs/zerolog"
)

func main() {

	logLevel, err := strconv.Atoi(os.Getenv("LOG_LEVEL"))
	if err != nil {
		logLevel = int(zerolog.InfoLevel) // default to INFO
	}
	zerolog.SetGlobalLevel(zerolog.Level(logLevel))

	cmd.Execute()
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
	// fmt.Printf("untagged in %v: %v\n", i, untagged.ImageDetails)
	// fmt.Printf("deet.RepositoryName: %v:%v Tags: %v\n", *deet.RepositoryName, *deet.ImageDigest, deet.ImageTags)
	// fmt.Printf("unt.ImageDetails: %v\n", unt.ImageDetails)
	// fmt.Printf("removeUntagged: %v\n", removeUntagged)

	// for _, image := range allImages {
	// 	_, err := json.MarshalIndent(image, "", "\t")
	// 	if err != nil {
	// 		log.Error().Err(err).Msg("failed to marshalIndent json")
	// 	}
	// 	// fmt.Printf("s: %v\n", string(s))

	// 	// if image.LastPushed.Before(time.Now().AddDate(0, 0, -(daysOld))) {
	// 	// 	fmt.Printf("%v is older than %v days old\n", image.FullImagePath, daysOld)
	// 	// }
	// 	for _, ecr := range keepers {
	// 		for _, deets := range ecr.ImageDetails {
	// 			// fmt.Printf("deets.ImageDigest: %v\n", *deets.ImageDigest)
	// 			deetsDigest := strings.Split(*deets.ImageDigest, ":")[1]
	// 			// fmt.Printf("deetsDigest: %v\n", deetsDigest)
	// 			// fmt.Printf("image.Digest: %v\n", image.Digest)
	// 			if deetsDigest == image.Digest {

	// 				// fmt.Printf("running image %v:%v pushed at %v\n", *deets.RepositoryName, deets.ImageTags, deets.ImagePushedAt)
	// 				continue
	// 			}
	// 		}
	// 	}
	// }

}
