This is the backend component of our CS492 project. If you want to mess around with the API, you can find it here

https://walrus-app-hcvlh.ondigitalocean.app/cs492-back2

# Endpoints

- `/playlist?id=playlist_id` returns metadata and analysis on a given playlist
- `/recs?id=playlist_id` returns metadata and analysis on the recommendations made using
a given playlist. The seeds can be customized with `seeds=id1,id2` and target features
can be selected using `feature=val`.
  - For example, `https://walrus-app-hcvlh.ondigitalocean.app/cs492-back2/recs?id=6p21dRudS9FmcyGvKWPq2R&seeds=6EMVcjvsBg8oF6Yb31ilEc&liveness=0.33`
  gives the recommendations using seed `6EMVcjvsBg8oF6Yb31ilEc` and a target liveness of 0.33

# Getting Started

To run the project, you will need to register an application on Spotify's end

https://developer.spotify.com/my-applications/

The client id and secret will need to be exported as env variables

```bash
export CLIENT_ID=client_id
export CLIENT_SECRET=client_secret 
```

If you have Go installed, you can directly run

```bash
go run main.go
```

Otherwise, with Docker, you can build and run the image with the id and secret inside a .env file

```bash
docker build -t cs492 .
docker run --env-file .env -d -p 8080:8080 cs492
```