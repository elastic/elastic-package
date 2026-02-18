#!/usr/bin/env python

import os

integrations_repo_path = "../../integrations"
packages_path = os.path.join(integrations_repo_path, "packages")

# Iterate through all packages
for package_path in os.listdir(packages_path):

    rally_path = os.path.join(packages_path, package_path, "_dev", "benchmark", "rally")

    # Find packages with a rally directory
    if os.path.isdir(rally_path):
        print("Package: " + package_path)

        benchmarks = os.listdir(rally_path)
        # List benchmarks
        for b in benchmarks:
            if not os.path.isdir(os.path.join(rally_path, b)):
                continue
            print("* " + b + "")

        print("")
