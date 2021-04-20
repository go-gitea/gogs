---
date: "2017-08-23T09:00:00+02:00"
title: "Installation depuis le code source"
slug: "install-from-source"
weight: 10
toc: false
draft: false
menu:
  sidebar:
    parent: "installation"
    name: "Code source"
    weight: 30
    identifier: "install-from-source"
---

# Installation depuis le code source

Nous ne couvrirons pas les bases de la configuration de Golang dans ce guide. Si vous ne savez pas comment démarrer un environnement fonctionnel, vous devrez suivre les [instructions d'installation](https://golang.org/doc/install) officielles.

**Attention**: La version 1.7 ou suppérieur de Go est nécessaire

## Téléchargement

Tout d'abord, vous devez récupérer le code source, la manière la plus simple est d'utiliser directement Go. Il suffit d'appeler les commandes suivantes pour récupérer le code source et passer au répertoire de travail.

```
go get -d -u code.gitea.io/gitea
cd $GOPATH/src/code.gitea.io/gitea
```

Maintenant, il est temps de décider quelle version de Gitea vous souhaitez compiler et installer. Actuellement, ils existent plusieurs options possibles. Si vous voulez compiler notre branche `master`, vous pouvez directement passer à la [section compilation](#compilation), cette branche représente la dernière version en cours de développement et n'a pas vocation à être utiliser en production.


Si vous souhaitez compiler la dernière version stable, utilisez les étiquettes ou les différentes branches disponibles. Vous pouvez voir les branches disponibles et comment utiliser cette branche avec ces commandes:

```
git branch -a
git checkout v{{< version >}}
```

Si vous souhaitez valider une demande d'ajout (_Pull request_), vous devez activer cette branche en premier :

```
git fetch origin pull/xyz/head:pr-xyz  # xyz is PR value
```

Enfin, vous pouvez directement utiliser les versions étiquettées (ex : `v{{< version >}}`). Pour utiliser les étiquettes, vous devez lister les étiquettes disponibles et choisir une étiquette spécifique avec les commandes suivantes :

```
git tag -l
git checkout v{{< version >}}
git checkout pr-xyz
```

## Compilation

Comme nous regroupons déjà toutes les bibliothèques requises pour compiler Gitea, vous pouvez continuer avec le processus de compilation lui-même. Nous fournissons diverses [tâches Make](https://github.com/go-gitea/gitea/blob/master/Makefile) pour rendre le processus de construction aussi simple que possible. <a href='{{< relref "doc/advanced/make.fr-fr.md" >}}'>Voyez ici comment obtenir Make</a>. Selon vos besoins, vous pourrez éventuellement ajouter diverses options de compilation, vous pouvez choisir entre ces options :

* `bindata`: Intègre toutes les ressources nécessaires à l'exécution d'une instance de Gitea, ce qui rend un déploiement facile car il n'est pas nécessaire de se préoccuper des fichiers supplémentaires.
* `pam`: Active la prise en charge de PAM (mLinux Pluggable Authentication Modules), très utile si vos utilisateurs doivent être authentifiés avec les comptes du système.

Il est temps de compiler le binaire, nous suggérons d'intégrer les ressources avec l'option de compilation `bindata`:

```
TAGS="bindata" make build
```

## Test

Après avoir suivi toutes les étapes, vous devriez avoir le binaire `gitea` dans votre répertoire courant. Dans un premier temps, vous pouvez tester qu'il fonctionne puis, dans un second temps, vous pouvez le copier dans la destination de votre choix. Lorsque vous lancez Gitea manuellement à partir de votre CLI, vous pouvez toujours le tuer en appuyant sur `Ctrl + C`.

```
./gitea web
```

## Il manque quelque chose ?

Est-ce que nous avons oublié quelque chose sur cette page ? N'hésitez pas à nous contacter sur notre [serveur Discord](https://discord.gg/Gitea), vous obtiendrez des réponses à toute vos questions assez rapidement.
