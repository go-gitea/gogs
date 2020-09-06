for file in $(ls *.yml)
    do mv $file "gitea_"$file
done