FROM node:20-alpine AS build
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm install
COPY . .
ARG VITE_API_URL=http://localhost:8080
ARG VITE_WS_URL=ws://localhost:8080/ws
ARG VITE_STREAM_URL=http://localhost:8090/stream
ENV VITE_API_URL=$VITE_API_URL VITE_WS_URL=$VITE_WS_URL VITE_STREAM_URL=$VITE_STREAM_URL
RUN npm run build

FROM nginx:alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
