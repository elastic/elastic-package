import argparse


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--list", required=True)
    parser.add_argument("--reference", required=False)
    parser.add_argument("--dashboard", required=False)
    parser.add_argument("--all_dashboards", action="store_true", required=False)

    args = parser.parse_args()

    lines = []
    with open(args.list, "r") as f:
        lines = f.readlines()
    dashboards_references = {}
    references_dashboards = {}

    for line in lines:
        fields = line.split(":")
        dashboard = fields[0]

        references = [r.strip() for r in fields[1].split(",")]

        dashboards_references[dashboard] = references

        for r in references:
            if r in references_dashboards:
                references_dashboards[r].append(dashboard)
            else:
                references_dashboards[r] = [dashboard]

    if args.reference:
        print(references_dashboards[args.reference])

    if args.dashboard:
        print(dashboards_references[args.dashboard])
        for ref in dashboards_references[args.dashboard]:
            number = len(references_dashboards[ref])
            print(f" - {ref}: {number}")

    print("")

    if args.all_dashboards:
        for dashboard in dashboards_references:
            print(f"Dashboard {dashboard}:")
            for ref in dashboards_references[dashboard]:
                number = len(references_dashboards[ref])
                print(f" - {ref}: {number}")
            print("")
