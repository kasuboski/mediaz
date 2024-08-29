generate: tmdb.schema.json prowlarr.schema.json
	go generate ./...

tmdb.schema.json:
	curl https://developer.themoviedb.org/openapi/64542913e1f86100738e227f | \
 	jq '.paths."/3/tv/{series_id}/season/{season_number}".get.responses."200".content."application/json".schema.properties._id."x-go-name" = "Identifier"' > tmdb.schema.json

prowlarr.schema.json:
	curl https://raw.githubusercontent.com/Prowlarr/Prowlarr/32d23d6636aebc4490e36215bebf1a786abdb46f/src/Prowlarr.Api.V1/openapi.json | \
	jq '.paths."/".get.parameters = null' > prowlarr.schema.json

clean:
	rm tmdb.schema.json
	rm prowlarr.schema.json
