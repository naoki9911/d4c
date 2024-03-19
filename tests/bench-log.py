import csv
import sys

diff_binary_csv = csv.writer(open("diff_binary_log.csv", "w"))
diff_file_csv = csv.writer(open("diff_file_log.csv", "w"))
patch_csv = csv.writer(open("patch_log.csv", "w"))
di3fs_csv = csv.writer(open("di3fs_log.csv", "w"))
merge_csv = csv.writer(open("merge_log.csv", "w"))
pull_csv = csv.writer(open("pull_log.csv", "w"))
pull_download_csv = csv.writer(open("pull_download_log.csv", "w"))

images=["apache", "mysql", "nginx", "postgres", "redis"]
for image in images:
    with open("{}-{}.log".format(image, sys.argv[1])) as f:
        reader = csv.reader(f)
        for row in reader:
            row.insert(0, image)
            if row[1] == "diff":
                if row[6] == "binary-diff":
                    diff_binary_csv.writerow(row)
                elif row[6] == "file-diff":
                    diff_file_csv.writerow(row)
                else:
                    print(row)
            elif row[1] == "patch":
                patch_csv.writerow(row)
            elif row[1] == "di3fs":
                di3fs_csv.writerow(row)
            elif row[1] == "merge":
                merge_csv.writerow(row)
            elif row[1] == "pull":
                pull_csv.writerow(row)
            elif row[1] == "pull-download":
                pull_download_csv.writerow(row)
            else:
                print(row)

