import json
import logging
import os
import threading
import time
from datetime import datetime

from kubernetes import client, config
from kubernetes.client.rest import ApiException
from jupyter_client.blocking import BlockingKernelClient

LOG_LEVEL = os.getenv("LOG_LEVEL", "INFO")

logging.basicConfig(level=LOG_LEVEL)
logger = logging.getLogger(__name__)


def update_cr_resource(
    namespace: str,
    cr_name: str,
    labels: dict | None = None,
    annotations: dict | None = None,
) -> None:
    """Update Kubernetes CRD labels or annotation with multiple key-value pairs."""
    if not labels and not annotations:
        logger.error("No labels or annotations provided to update.")
        return

    # Load kubeconfig to configure Kubernetes client
    try:
        config.load_incluster_config()
    except config.ConfigException:
        config.load_kube_config()

    api_instance = client.CustomObjectsApi()

    # Fetch the current CRD object
    crd = api_instance.get_namespaced_custom_object(
        group="jupyrator.org",
        version="v1",
        namespace=namespace,
        plural="kernelmanagers",
        name=cr_name,
    )

    if labels:
        # Retrieve existing labels, if any
        _labels = crd.get("metadata", {}).get("labels", {})
        # Update labels with new key-value pairs
        _labels.update(labels)
        # Set the updated labels back to the CRD
        crd["metadata"]["labels"] = _labels

    if annotations:
        # Retrieve existing annotations, if any
        _annotations = crd.get("metadata", {}).get("annotations", {})
        # Update annotations with new key-value pairs
        _annotations.update(annotations)
        # Set the updated annotations back to the CRD
        crd["metadata"]["annotations"] = _annotations

    # Replace the CRD with updated labels
    api_instance.replace_namespaced_custom_object(
        group="jupyrator.org",
        version="v1",
        namespace=namespace,
        plural="kernelmanagers",
        name=cr_name,
        body=crd,
    )
    logger.info(f"Successfully updated labels for {cr_name}.")


class KernelMonitor:
    def __init__(
        self,
        namespace: str,
        cr_name: str,
        connection_info: dict,
        idle_timeout: int,
        culling_interval: int,
    ):
        self.namespace = namespace
        self.cr_name = cr_name
        self.connection_info = connection_info
        self.idle_timeout = idle_timeout
        self.culling_interval = culling_interval

        # Initialize kernel client and monitor last activity time
        self.client = BlockingKernelClient()
        self.client.load_connection_info(self.connection_info)
        # Sidecar should wait for kernel to be ready
        logger.info("Wait for kernel ready...")
        self.client.wait_for_ready()
        self.client.start_channels()
        logger.info("Kernel is ready, starting monitor...")

        # Set last activity timestamp to current time if kernel is ready
        self.last_activity_timestamp = time.time()
        self.lock = threading.Lock()

    def update_last_activity(self):
        with self.lock:
            self.last_activity_timestamp = time.time()
        annotations = {
            "jupyrator.org/lastActivityTime": str(datetime.now()),
        }
        update_cr_resource(
            namespace=self.namespace, cr_name=self.cr_name, annotations=annotations
        )

    def monitor_activity(self) -> None:
        """Monitor kernel activity and idle time."""
        while True:
            try:
                msg = self.client.get_iopub_msg(timeout=1)
                if (
                    msg
                    and msg["header"]["msg_type"] == "status"
                    and msg["content"]["execution_state"] == "busy"
                ):
                    # Kernel is busy, update last activity timestamp
                    logger.debug("Kernel is busy, updating last activity timestamp.")
                    self.update_last_activity()
            except Exception:
                logger.debug("No message received from kernel.")
                pass

    def start(self):
        threading.Thread(target=self.monitor_activity, daemon=True).start()
        while True:
            current_time = time.time()
            with self.lock:
                idle_time = current_time - self.last_activity_timestamp
            if idle_time > self.idle_timeout:
                labels = {"jupyrator.org/kernelmanager-idle": "true"}
                try:
                    update_cr_resource(
                        namespace=self.namespace, cr_name=self.cr_name, labels=labels
                    )
                except ApiException as e:
                    logger.error(f"Failed to update CR label: {e}")
            time.sleep(self.culling_interval)


if __name__ == "__main__":
    # Load environment variables
    name = os.getenv("NAME")
    namespace = os.getenv("NAMESPACE")
    idle_timeout = int(os.getenv("IDLE_TIMEOUT", 3600))
    culling_interval = int(os.getenv("CULLING_INTERVAL", 60))

    if namespace is None:
        raise ValueError("NAMESPACE is not set in environment variables.")

    if name is None:
        raise ValueError("CRD NAME is not set in environment variables.")

    logger.info("Kernel name: %s", name)
    logger.info("Namespace: %s", namespace)
    logger.info("Kernel idle timeout: %s", idle_timeout)
    logger.info("Kernel culling interval: %s", culling_interval)

    with open(f"/tmp/{name}.json") as f:
        connection_info = json.load(f)
    logger.info("Kernel connection info %s", connection_info)

    monitor = KernelMonitor(
        namespace, name, connection_info, idle_timeout, culling_interval
    )
    monitor.start()
