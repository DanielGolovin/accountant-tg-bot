set -a
source .env
set +a

docker build -t accountant-bot -f Dockerfile.prod .
docker save accountant-bot -o accountant-bot.tar

ssh $REMOTE_USER@$REMOTE_SERVER_IP "\
mkdir -p /home/accountant-bot"

scp accountant-bot.tar $REMOTE_USER@$REMOTE_SERVER_IP:/home/accountant-bot/accountant-bot.tar
scp docker-compose-production.yml $REMOTE_USER@$REMOTE_SERVER_IP:/home/accountant-bot/docker-compose.yml
scp .env $REMOTE_USER@$REMOTE_SERVER_IP:/home/accountant-bot/.env

ssh $REMOTE_USER@$REMOTE_SERVER_IP "\
cd /home/accountant-bot && \
docker load -i accountant-bot.tar && \
docker compose up -d && \
docker system prune -f"