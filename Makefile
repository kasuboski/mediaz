generate: tmdb.schema.json
	go generate ./...

tmdb.schema.json:
	curl https://developer.themoviedb.org/openapi/64542913e1f86100738e227f | \
 	jq '.paths."/3/tv/{series_id}/season/{season_number}".get.responses."200".content."application/json".schema.properties._id."x-go-name" = "Identifier"' > tmdb.schema.json

clean:
	rm tmdb.schema.json
