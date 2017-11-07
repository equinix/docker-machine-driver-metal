# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/).
This project adheres to [Semantic Versioning](http://semver.org/spec/v2.0.0.html).


## [0.1.5] - 2017-11-07
### Added
- Ability to pass plan as either `baremetal_T` or `typeT`
- Verify plan is valid

### Fixed
- Build against latest packngo api break

## [0.1.4] - 2017-07-21
### Added
- Expanded the list of valid OperatingSystems
- Add RancherOS ssh username
- Minor tweaks to Driver structure to be consistent with upstream machine drivers
- Ability to pass in userdata

### Changed
- Default os is now `ubuntu_16_04` instead of `ubuntu_14_04`
- Default plan is now `baremetal_0` instead of `baremetal_1`

## [0.1.3] - 2016-12-30
### Fixed
- Build against latest packngo api break

### Changed
- Update minimum supported version of docker-machine to v0.8.2+

## [0.1.2] - 2016-03-03
### Changed
- 404 responses of a DELETE call are no longer treated as an error

## [0.1.1] - 2016-03-03
### Changed
- Update minimum supported version of docker-machine to v0.5.5+

## [0.1.0] - 2015-12-05
### Fixed
- Local storage of generated ssh keys

## [0.0.2] - 2015-11-19
Nothing done, NOP release.

## [0.0.1] - 2015-11-19
### Added
- Initial release, has basic device creation working
