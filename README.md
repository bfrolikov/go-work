# go-work
* A cron-like app that runs jobs (defined by their script paths) at specified intervals
* Jobs are stored in a PostgreSQL database
* Supports multiple concurrent jobs runners
* Has a RESTful web interface for adding/removing/listing jobs
#### This project requires finishing touches
**What's completed**:
- [x] Job storage
- [x] Job scheduler
- [x] Command line interface for starting and stopping the app
- [x] RESTful web interface for adding/removing/listing jobs
- [x] OpenAPI 3.0 specification
- [x] Testing
- [x] Dockerfile and docker-compose.yml
- [x] CI/CD

**What's left**:
- [ ] Create indices in the database
- [ ] Add a comprehensive description and usage guide