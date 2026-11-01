# Selector-less Services backed by manual Endpoints pointing at the node IP.
# PostgreSQL and Redis run on the host (docker compose), NOT inside k3s, so the
# cluster can be rebuilt without touching data.
#
# __NODE_IP__ is replaced by deploy/k3s/scripts/deploy-app.sh with the node's
# private/internal IP. Do not apply this template directly.
apiVersion: v1
kind: Service
metadata:
  name: dragz-postgres
  namespace: dragz-prod
spec:
  ports:
    - name: postgres
      protocol: TCP
      port: 5432
      targetPort: 5432
---
apiVersion: v1
kind: Endpoints
metadata:
  name: dragz-postgres
  namespace: dragz-prod
subsets:
  - addresses:
      - ip: __NODE_IP__
    ports:
      - name: postgres
        protocol: TCP
        port: 5432
---
apiVersion: v1
kind: Service
metadata:
  name: dragz-redis
  namespace: dragz-prod
spec:
  ports:
    - name: redis
      protocol: TCP
      port: 6379
      targetPort: 6379
---
apiVersion: v1
kind: Endpoints
metadata:
  name: dragz-redis
  namespace: dragz-prod
subsets:
  - addresses:
      - ip: __NODE_IP__
    ports:
      - name: redis
        protocol: TCP
        port: 6379
