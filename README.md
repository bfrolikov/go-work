# go-work
* A cron-like app that runs jobs (defined by their script paths) at specified intervals
* Jobs are stored in a PostgreSQL database
* Supports multiple concurrent jobs runners
* Has a REST-ful web inteface for adding/removing/listing jobs
#### This project is a WIP, see the `development` branch for updates
**TODO**:
- [x] Job storage
- [x] Job scheduler
- [x] Command line interface for starting and stopping the app
- [x] RESTful web interface for adding/removing/listing jobs
- [ ] OpenAPI 3.0 specification
- [ ] Testing
- [ ] Dockerfile and docker-compose.yml
- [ ] CI/CD before merging into `master`
