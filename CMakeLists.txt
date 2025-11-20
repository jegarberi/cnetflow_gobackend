cmake_minimum_required(VERSION 3.10)
project(cnetflow_gobackend)

# Set Go binary path
find_program(GO_EXECUTABLE go)
if(NOT GO_EXECUTABLE)
    message(FATAL_ERROR "Go executable not found. Please install Go.")
endif()

# Set build output directory
set(OUTPUT_DIR ${CMAKE_BINARY_DIR}/bin)
set(BINARY_NAME cnetflow_gobackend)

# Go build flags for CentOS 7
set(GO_BUILD_FLAGS
    -ldflags "-s -w"
    -trimpath
)

# Set GOOS and GOARCH for Linux
set(GOOS linux)
set(GOARCH amd64)

# Custom target to build the Go application
add_custom_target(go_build ALL
    COMMAND ${CMAKE_COMMAND} -E env
        GOOS=${GOOS}
        GOARCH=${GOARCH}
        CGO_ENABLED=1
        ${GO_EXECUTABLE} build ${GO_BUILD_FLAGS} -o ${OUTPUT_DIR}/${BINARY_NAME}
    WORKING_DIRECTORY ${CMAKE_SOURCE_DIR}
    COMMENT "Building Go application for CentOS 7..."
)

# Custom target to run go mod download
add_custom_target(go_dependencies
    COMMAND ${GO_EXECUTABLE} mod download
    WORKING_DIRECTORY ${CMAKE_SOURCE_DIR}
    COMMENT "Downloading Go dependencies..."
)

# Make go_build depend on go_dependencies
add_dependencies(go_build go_dependencies)

# Install the binary
install(PROGRAMS ${OUTPUT_DIR}/${BINARY_NAME}
    DESTINATION bin
)

# Install static files
install(DIRECTORY ${CMAKE_SOURCE_DIR}/static
    DESTINATION share/${BINARY_NAME}
    PATTERN ".*" EXCLUDE
)

# Install GeoIP database (optional)
if(EXISTS ${CMAKE_SOURCE_DIR}/GeoLite2-City.mmdb)
    install(FILES ${CMAKE_SOURCE_DIR}/GeoLite2-City.mmdb
        DESTINATION share/${BINARY_NAME}
    )
else()
    message(WARNING "GeoLite2-City.mmdb not found in source tree; package will be built without the GeoIP database.")
endif()

# Install SQL init file if present (optional)
if(EXISTS ${CMAKE_SOURCE_DIR}/init.sql)
    install(FILES ${CMAKE_SOURCE_DIR}/init.sql
        DESTINATION share/${BINARY_NAME}
    )
else()
    message(STATUS "init.sql not found; skipping installation of database init script.")
endif()

# CPack configuration for RPM packaging (CentOS 7)
set(CPACK_GENERATOR "RPM")
set(CPACK_PACKAGE_NAME "cnetflow-gobackend")
set(CPACK_PACKAGE_VERSION "1.0.0")
set(CPACK_PACKAGE_RELEASE 1)
set(CPACK_PACKAGE_CONTACT "admin@example.com")
set(CPACK_PACKAGE_VENDOR "CNetFlow")
set(CPACK_PACKAGE_DESCRIPTION_SUMMARY "CNetFlow Go Backend - Network flow analysis backend")
set(CPACK_PACKAGE_DESCRIPTION "Backend service for CNetFlow network flow analysis and visualization")

# RPM specific settings
set(CPACK_RPM_PACKAGE_LICENSE "Proprietary")
set(CPACK_RPM_PACKAGE_GROUP "Applications/Internet")
set(CPACK_RPM_PACKAGE_REQUIRES "glibc >= 2.17")
set(CPACK_RPM_PACKAGE_ARCHITECTURE "x86_64")
set(CPACK_RPM_COMPRESSION_TYPE "xz")

# Set installation prefix
set(CPACK_PACKAGING_INSTALL_PREFIX "/opt/cnetflow")

include(CPack)
