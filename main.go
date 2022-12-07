package main

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"go.uber.org/zap"
	"golang.org/x/oauth2/clientcredentials"
)

func getCentroid(feats []*spotify.AudioFeatures) spotify.AudioFeatures {
	n := float32(len(feats))
	avgFeat := spotify.AudioFeatures{}

	for _, feat := range feats {
		avgFeat.Acousticness += feat.Acousticness / n
		avgFeat.Danceability += feat.Danceability / n
		avgFeat.Energy += feat.Energy / n
		avgFeat.Instrumentalness += feat.Instrumentalness / n
		avgFeat.Liveness += feat.Liveness / n
		// avgFeat.Loudness += feat.Loudness / n
		avgFeat.Speechiness += feat.Speechiness / n
		avgFeat.Valence += feat.Valence / n
	}

	return avgFeat
}

func getMSE(feats []*spotify.AudioFeatures, centroid spotify.AudioFeatures) float64 {
	n := float64(len(feats))
	var diff float64

	for _, feat := range feats {
		var cur float64
		cur += math.Pow(float64(centroid.Acousticness-feat.Acousticness), 2)
		cur += math.Pow(float64(centroid.Danceability-feat.Danceability), 2)
		cur += math.Pow(float64(centroid.Energy-feat.Energy), 2)
		cur += math.Pow(float64(centroid.Instrumentalness-feat.Instrumentalness), 2)
		cur += math.Pow(float64(centroid.Liveness-feat.Liveness), 2)
		// cur += math.Pow(float64(centroid.Loudness-feat.Loudness), 2)
		cur += math.Pow(float64(centroid.Speechiness-feat.Speechiness), 2)
		cur += math.Pow(float64(centroid.Valence-feat.Valence), 2)
		diff += cur
	}

	return diff / n
}

type Track struct {
	ID         string
	Name       string
	Artists    []string
	Popularity int
	ImageURL   string
}

type PlaylistInfo struct {
	Tracks []Track

	Centroid spotify.AudioFeatures
	MSE      float64
}

type reqHandler func(http.ResponseWriter, *http.Request)

func getPlaylistFunc(sc *spotify.Client, logger *zap.Logger) reqHandler {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		id := r.URL.Query().Get("id")

		logger.Debug("got request for playlist", zap.String("id", id))

		items, err := sc.GetPlaylistItems(ctx, spotify.ID(id))
		if err != nil {
			logger.Debug("failed to get playlist", zap.Error(err))
			io.WriteString(w, "failed to get playlist")
			return
		}

		ids := []spotify.ID{}
		tracks := []Track{}

		for _, item := range items.Items {
			if item.Track.Track == nil {
				logger.Debug("skipped track cause missing information", zap.Any("item", item))
				continue
			}

			ids = append(ids, item.Track.Track.ID)

			artists := make([]string, 0)
			for _, artist := range item.Track.Track.Artists {
				artists = append(artists, artist.Name)
			}

			newTrack := Track{
				ID:         item.Track.Track.ID.String(),
				Name:       item.Track.Track.Name,
				Artists:    artists,
				Popularity: item.Track.Track.Popularity,
			}
			if len(item.Track.Track.Album.Images) > 0 {
				newTrack.ImageURL = item.Track.Track.Album.Images[0].URL
			}

			tracks = append(tracks, newTrack)
		}

		feats, err := sc.GetAudioFeatures(ctx, ids...)
		if err != nil {
			logger.Debug("failed to get audio features", zap.Error(err))
			io.WriteString(w, "failed to get audio features")
			return
		}

		avgFeat := getCentroid(feats)
		diff := getMSE(feats, avgFeat)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Add("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(PlaylistInfo{
			Tracks:   tracks,
			Centroid: avgFeat,
			MSE:      diff,
		})
	}
}

func getTargetAttributes(r *http.Request) *spotify.TrackAttributes {
	trackAttributes := spotify.NewTrackAttributes()

	// So tempted to use reflection here, but it should be fine.

	tAcousticness := r.URL.Query().Get("acousticness")
	vAcousticness, err := strconv.ParseFloat(tAcousticness, 64)
	if err == nil {
		trackAttributes.TargetAcousticness(vAcousticness)
	}

	tDanceability := r.URL.Query().Get("danceability")
	vDanceability, err := strconv.ParseFloat(tDanceability, 64)
	if err == nil {
		trackAttributes.TargetDanceability(vDanceability)
	}

	tEnergy := r.URL.Query().Get("energy")
	vEnergy, err := strconv.ParseFloat(tEnergy, 64)
	if err == nil {
		trackAttributes.TargetEnergy(vEnergy)
	}

	tInstrumentalness := r.URL.Query().Get("instrumentalness")
	vInstrumentalness, err := strconv.ParseFloat(tInstrumentalness, 64)
	if err == nil {
		trackAttributes.TargetInstrumentalness(vInstrumentalness)
	}

	tLiveness := r.URL.Query().Get("liveness")
	vLiveness, err := strconv.ParseFloat(tLiveness, 64)
	if err == nil {
		trackAttributes.TargetLiveness(vLiveness)
	}

	tSpeechiness := r.URL.Query().Get("speechiness")
	vSpeechiness, err := strconv.ParseFloat(tSpeechiness, 64)
	if err == nil {
		trackAttributes.TargetSpeechiness(vSpeechiness)
	}

	tValence := r.URL.Query().Get("valence")
	vValence, err := strconv.ParseFloat(tValence, 64)
	if err == nil {
		trackAttributes.TargetValence(vValence)
	}

	return trackAttributes
}

func getRecsFunc(sc *spotify.Client, logger *zap.Logger) reqHandler {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		playlistID := r.URL.Query().Get("id")

		logger.Debug("got request for playlist recommendations", zap.String("id", playlistID))

		items, err := sc.GetPlaylistItems(ctx, spotify.ID(playlistID))
		if err != nil {
			logger.Debug("failed to get playlist", zap.Error(err))
			io.WriteString(w, "failed to get playlist")
			return
		}

		ids := []spotify.ID{}

		seedIDStr := r.URL.Query().Get("seeds")
		if seedIDStr == "" {
			// Randomly select 5 tracks from the playlist as seed.
			idxs := rand.Perm(len(items.Items))
			if len(idxs) > 5 {
				idxs = idxs[:5]
			}

			for _, idx := range idxs {
				ids = append(ids, items.Items[idx].Track.Track.ID)
			}
		} else {
			// Otherwise, use provided seed string.
			seedIDs := strings.Split(seedIDStr, ",")
			if len(seedIDs) > 5 {
				seedIDs = seedIDs[:5]
			}

			for _, seedID := range seedIDs {
				ids = append(ids, spotify.ID(seedID))
			}
		}

		logger.Debug("made recommendations using songs as seeds", zap.Any("seeds", ids))

		recs, err := sc.GetRecommendations(ctx, spotify.Seeds{Tracks: ids}, getTargetAttributes(r))
		if err != nil {
			logger.Debug("failed to get recommendations", zap.Error(err))
			io.WriteString(w, "failed to get recommendations")
			return
		}

		recIDs := []spotify.ID{}
		for _, rec := range recs.Tracks {
			recIDs = append(recIDs, rec.ID)
		}

		recItems, err := sc.GetTracks(ctx, recIDs)
		if err != nil {
			logger.Debug("failed to get tracks", zap.Error(err))
			io.WriteString(w, "failed to get tracks")
			return
		}

		tracks := []Track{}
		for _, item := range recItems {
			artists := make([]string, 0)
			for _, artist := range item.Artists {
				artists = append(artists, artist.Name)
			}

			newTrack := Track{
				ID:         item.ID.String(),
				Name:       item.Name,
				Artists:    artists,
				Popularity: item.Popularity,
			}
			if len(item.Album.Images) > 0 {
				newTrack.ImageURL = item.Album.Images[0].URL
			}

			tracks = append(tracks, newTrack)
		}

		feats, err := sc.GetAudioFeatures(ctx, recIDs...)
		if err != nil {
			logger.Debug("failed to get audio features", zap.Error(err))
			io.WriteString(w, "failed to get audio features")
			return
		}

		avgFeat := getCentroid(feats)
		diff := getMSE(feats, avgFeat)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Add("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(PlaylistInfo{
			Tracks:   tracks,
			Centroid: avgFeat,
			MSE:      diff,
		})
	}
}

func main() {
	ctx := context.Background()
	config := &clientcredentials.Config{
		ClientID:     os.Getenv("CLIENT_ID"),
		ClientSecret: os.Getenv("CLIENT_SECRET"),
		TokenURL:     spotifyauth.TokenURL,
	}
	logger, _ := zap.NewDevelopment()

	httpClient := config.Client(ctx)
	client := spotify.New(httpClient)

	logger.Debug("spotify client started")

	http.HandleFunc("/playlist", getPlaylistFunc(client, logger))
	http.HandleFunc("/recs", getRecsFunc(client, logger))

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		logger.Fatal("failed to listen and serve", zap.Error(err))
	}
}
